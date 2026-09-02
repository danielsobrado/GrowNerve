package migrations

import _ "embed"

type Migration struct {
	Version int
	Up      string
	Down    string
}

//go:embed 000001_initial.up.sql
var initialUp string

//go:embed 000001_initial.down.sql
var initialDown string

//go:embed 000002_outbox_idempotency.up.sql
var outboxIdempotencyUp string

//go:embed 000002_outbox_idempotency.down.sql
var outboxIdempotencyDown string

var All = []Migration{
	{Version: 1, Up: initialUp, Down: initialDown},
	{Version: 2, Up: outboxIdempotencyUp, Down: outboxIdempotencyDown},
}

const LatestVersion = 2
