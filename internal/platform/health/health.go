// Package health serves /health and lets a binary probe its own endpoint, which is
// how Docker healthchecks run inside distroless images that have no curl.
package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/httpx"
)

// Check reports whether one dependency is usable.
type Check func(ctx context.Context) error

type result struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Handler runs every check with a short timeout. It answers 200 when all pass and
// 503 otherwise; failure detail names the dependency but not its error text.
func Handler(checks map[string]Check) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		res := result{Status: "ok", Checks: map[string]string{}}
		for name, check := range checks {
			if err := check(ctx); err != nil {
				res.Status = "unavailable"
				res.Checks[name] = "fail"
				continue
			}
			res.Checks[name] = "ok"
		}
		status := http.StatusOK
		if res.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		httpx.WriteJSON(w, status, res)
	})
}

// Probe GETs url and returns an error unless it answers 200.
func Probe(url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url) //nolint:noctx // one-shot CLI probe with a client timeout
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("probe %s: status %d", url, resp.StatusCode)
	}
	return nil
}
