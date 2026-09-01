package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const correlationKey contextKey = "correlation_id"

type Options struct {
	AllowedOrigins    []string
	TrustedProxyCIDRs []string
	Logger            *slog.Logger
	ReadLimit         RateLimit
	WriteLimit        RateLimit
}

func CorrelationID(ctx context.Context) string {
	value, _ := ctx.Value(correlationKey).(string)
	return value
}

func Chain(next http.Handler, options Options) http.Handler {
	var readLimiter, writeLimiter *Limiter
	if options.ReadLimit.Rate > 0 {
		readLimiter = NewLimiter(options.ReadLimit)
	}
	if options.WriteLimit.Rate > 0 {
		writeLimiter = NewLimiter(options.WriteLimit)
	}
	trustedProxies := parseTrustedProxyCIDRs(options.TrustedProxyCIDRs)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		correlationID := request.Header.Get("X-Correlation-ID")
		if _, err := uuid.Parse(correlationID); err != nil {
			correlationID = uuid.NewString()
		}
		request = request.WithContext(context.WithValue(request.Context(), correlationKey, correlationID))
		request.Header.Set("X-Correlation-ID", correlationID)
		writer.Header().Set("X-Correlation-ID", correlationID)
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		origin := request.Header.Get("Origin")
		if origin != "" && slices.Contains(options.AllowedOrigins, origin) {
			writer.Header().Set("Access-Control-Allow-Origin", origin)
			writer.Header().Set("Vary", "Origin")
			writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, If-Match, If-None-Match, X-Correlation-ID, X-Farm-Version")
			writer.Header().Set("Access-Control-Expose-Headers", "ETag, X-Correlation-ID, X-Farm-Version")
			writer.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
		}
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		limiter := readLimiter
		if isMutating(request.Method) {
			limiter = writeLimiter
		}
		if limiter != nil && !limiter.Allow(clientKeyWithProxies(request, trustedProxies)) {
			writer.Header().Set("Retry-After", strconv.Itoa(int(limiter.RetryAfter().Seconds()+1)))
			if options.Logger != nil {
				options.Logger.Warn("http_rate_limited", "correlation_id", correlationID, "path", request.URL.Path, "method", request.Method)
			}
			http.Error(writer, "too many requests", http.StatusTooManyRequests)
			return
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				if options.Logger != nil {
					options.Logger.Error("http_panic", "correlation_id", correlationID, "panic", recovered)
				}
				http.Error(writer, "internal server error", http.StatusInternalServerError)
			}
			if options.Logger != nil {
				options.Logger.Info("http_request", "method", request.Method, "path", request.URL.Path, "correlation_id", correlationID, "duration_ms", time.Since(started).Milliseconds())
			}
		}()
		next.ServeHTTP(writer, request)
	})
}
