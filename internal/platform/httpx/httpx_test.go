package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter(buf *bytes.Buffer, h http.HandlerFunc) http.Handler {
	log := slog.New(slog.NewJSONHandler(buf, nil))
	r := chi.NewRouter()
	r.Use(RequestID, Recoverer(log), AccessLog(log))
	r.Get("/things/{id}", h)
	return r
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) Envelope {
	t.Helper()
	var env Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env
}

func TestClientErrorEnvelope(t *testing.T) {
	var logs bytes.Buffer
	r := newTestRouter(&logs, func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, slog.New(slog.NewJSONHandler(&logs, nil)), ErrNotFound)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/42", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
	env := decode(t, rec)
	assert.Equal(t, "NOT_FOUND", env.ErrorCode)
	assert.NotEmpty(t, env.RequestID)
	assert.Equal(t, env.RequestID, rec.Header().Get("X-Request-ID"))
}

func TestInternalErrorHidesCause(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	r := newTestRouter(&logs, func(w http.ResponseWriter, req *http.Request) {
		WriteError(w, req, log, errors.New("pq: relation users does not exist"))
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/1", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "relation users")
	assert.Equal(t, "INTERNAL", decode(t, rec).ErrorCode)
	assert.Contains(t, logs.String(), "relation users", "cause is logged server-side")
	assert.Contains(t, logs.String(), `"route":"/things/{id}"`)
}

func TestRecovererReturnsEnvelopeWithoutStack(t *testing.T) {
	var logs bytes.Buffer
	r := newTestRouter(&logs, func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/1", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "goroutine")
	assert.NotContains(t, rec.Body.String(), "boom")
	assert.Contains(t, logs.String(), "goroutine", "stack is logged server-side")
}

func TestRequestIDPropagation(t *testing.T) {
	var logs bytes.Buffer
	r := newTestRouter(&logs, func(w http.ResponseWriter, req *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"id": RequestIDFrom(req.Context())})
	})

	req := httptest.NewRequest(http.MethodGet, "/things/1", nil)
	req.Header.Set("X-Request-ID", "caddy-abc-12345")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, "caddy-abc-12345", rec.Header().Get("X-Request-ID"))

	req = httptest.NewRequest(http.MethodGet, "/things/1", nil)
	req.Header.Set("X-Request-ID", "bad id with spaces <script>")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.NotEqual(t, "bad id with spaces <script>", rec.Header().Get("X-Request-ID"))
	assert.Len(t, rec.Header().Get("X-Request-ID"), 36)
}

func TestAccessLogUsesRoutePatternNotRawPath(t *testing.T) {
	var logs bytes.Buffer
	r := newTestRouter(&logs, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/0912345678?email=a@b.vn", nil))

	assert.NotContains(t, logs.String(), "0912345678")
	assert.NotContains(t, logs.String(), "a@b.vn")
	assert.Contains(t, logs.String(), `"status":204`)
}
