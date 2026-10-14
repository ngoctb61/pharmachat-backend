// Package httpx holds HTTP plumbing shared by services: the error envelope
// {errorCode, message, requestId}, request IDs, panic recovery and access logs.
// Stack traces and internal error text never reach the client (constitution Art. 3).
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Error is a client-facing failure. Message is shown to users, so it is Vietnamese
// and never contains internal details.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// Envelope is the JSON body of every error response.
type Envelope struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// Common errors.
var (
	ErrInternal = &Error{Status: http.StatusInternalServerError, Code: "INTERNAL", Message: "Loi he thong, vui long thu lai sau"}
	ErrNotFound = &Error{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "Khong tim thay"}
)

type ctxKey struct{}

// RequestIDFrom returns the request ID stored by the RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9\-]{8,64}$`)

// RequestID accepts a well-formed inbound X-Request-ID (e.g. from Caddy) or mints one,
// and echoes it on the response.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the error envelope. Errors that are not *Error are logged with
// their cause and reported to the client as ErrInternal.
func WriteError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var e *Error
	if !errors.As(err, &e) {
		log.ErrorContext(r.Context(), "unhandled error",
			"request_id", RequestIDFrom(r.Context()), "route", routePattern(r), "err", err.Error())
		e = ErrInternal
	}
	WriteJSON(w, e.Status, Envelope{ErrorCode: e.Code, Message: e.Message, RequestID: RequestIDFrom(r.Context())})
}

// Recoverer turns a panic into a 500 envelope and logs the stack server-side.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(rec)
					}
					log.ErrorContext(r.Context(), "panic recovered",
						"request_id", RequestIDFrom(r.Context()), "route", routePattern(r),
						"panic", rec, "stack", string(debug.Stack()))
					WriteJSON(w, ErrInternal.Status, Envelope{
						ErrorCode: ErrInternal.Code, Message: ErrInternal.Message, RequestID: RequestIDFrom(r.Context()),
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog logs one line per request. It records the route pattern, not the raw
// path, so IDs and query strings with personal data stay out of the logs.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			log.InfoContext(r.Context(), "http request",
				"request_id", RequestIDFrom(r.Context()), "method", r.Method, "route", routePattern(r),
				"status", sw.status, "duration_ms", time.Since(start).Milliseconds())
		})
	}
}

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
