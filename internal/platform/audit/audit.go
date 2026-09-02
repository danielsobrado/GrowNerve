// Package audit durably records security-relevant actions. Recording is
// deliberately best-effort at the call site and never blocks the request path,
// but a failure to record is logged loudly rather than swallowed.
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdanielsobrado/grownerve/internal/farm"
	"github.com/jdanielsobrado/grownerve/internal/platform/database/gen"
)

const (
	queueDepth           = 512
	auditWriteTimeout    = 5 * time.Second
	shutdownDrainTimeout = 5 * time.Second
)

// Recorder writes audit entries to PostgreSQL through a bounded queue, so a slow
// database cannot add latency to an operator's command.
type Recorder struct {
	queries *gen.Queries
	logger  *slog.Logger
	entries chan farm.AuditEntry
	done    chan struct{}
}

func NewRecorder(pool *pgxpool.Pool, logger *slog.Logger) *Recorder {
	return &Recorder{
		queries: gen.New(pool), logger: logger,
		entries: make(chan farm.AuditEntry, queueDepth), done: make(chan struct{}),
	}
}

// Start drains the queue until the context is cancelled.
func (recorder *Recorder) Start(ctx context.Context) {
	go func() {
		defer close(recorder.done)
		for {
			select {
			case <-ctx.Done():
				recorder.drainOnShutdown()
				return
			case entry := <-recorder.entries:
				recorder.write(ctx, entry)
			}
		}
	}()
}

func (recorder *Recorder) drainOnShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownDrainTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			if pending := len(recorder.entries); pending > 0 {
				recorder.logger.Error("audit_shutdown_incomplete", "pending", pending)
			}
			return
		case entry := <-recorder.entries:
			recorder.write(ctx, entry)
		default:
			return
		}
	}
}

// Wait blocks until the drain loop has finished.
func (recorder *Recorder) Wait() { <-recorder.done }

func (recorder *Recorder) Record(_ context.Context, entry farm.AuditEntry) {
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now().UTC()
	}
	select {
	case recorder.entries <- entry:
	default:
		recorder.logger.Error("audit_queue_full", "action", entry.Action, "target", entry.TargetID)
	}
}

func (recorder *Recorder) write(parent context.Context, entry farm.AuditEntry) {
	ctx, cancel := context.WithTimeout(parent, auditWriteTimeout)
	defer cancel()

	detail := make(map[string]any, len(entry.Detail)+1)
	for key, value := range entry.Detail {
		detail[key] = value
	}
	// The actor is recorded as a label rather than a foreign key: identities can
	// come from an external provider that has no row in the users table, and
	// losing the audit entry would be worse than losing the join.
	detail["actor"] = entry.Actor
	encoded, err := json.Marshal(detail)
	if err != nil {
		recorder.logger.Error("audit_detail_invalid", "error", err, "action", entry.Action, "target", entry.TargetID)
		encoded, _ = json.Marshal(map[string]any{"actor": entry.Actor})
	}
	params := gen.InsertAuditEntryParams{
		Action:     entry.Action,
		TargetType: entry.TargetType,
		OccurredAt: pgtype.Timestamptz{Time: entry.OccurredAt.UTC(), Valid: true},
		Detail:     encoded,
	}
	var targetID pgtype.UUID
	if entry.TargetID != "" && targetID.Scan(entry.TargetID) == nil {
		params.TargetID = targetID
	}
	var correlationID pgtype.UUID
	if entry.CorrelationID != "" && correlationID.Scan(entry.CorrelationID) == nil {
		params.CorrelationID = correlationID
	}
	if err := recorder.queries.InsertAuditEntry(ctx, params); err != nil {
		recorder.logger.Error("audit_write_failed", "error", err, "action", entry.Action, "target", entry.TargetID)
	}
}

// LogRecorder writes audit entries to the structured log. It is the fallback for
// deployments without a database and keeps the audit trail present rather than
// silently absent.
type LogRecorder struct{ Logger *slog.Logger }

func (recorder LogRecorder) Record(_ context.Context, entry farm.AuditEntry) {
	recorder.Logger.Info("audit",
		"actor", entry.Actor, "action", entry.Action, "target_type", entry.TargetType,
		"target_id", entry.TargetID, "correlation_id", entry.CorrelationID, "detail", entry.Detail)
}
