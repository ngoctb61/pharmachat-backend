package auth

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/dbtest"
	"github.com/ngoctb61/pharmachat-backend/internal/platform/httpx"
)

func TestLoginHandler(t *testing.T) {
	dbtest.Require(t, env)
	f := newUser(t, "Admin")
	ownerExec(t, "UPDATE users SET is_active = false WHERE id = $1", newUser(t).userID)
	h := httpx.RequestID(http.HandlerFunc(NewHandler(newService(t), slog.New(slog.NewTextHandler(io.Discard, nil))).Login))

	cases := map[string]struct {
		body       string
		wantStatus int
		wantCode   string
	}{
		"ok":               {`{"email":"` + f.email + `","password":"` + testPassword + `"}`, 200, ""},
		"wrong password":   {`{"email":"` + f.email + `","password":"nope"}`, 401, "INVALID_CREDENTIALS"},
		"malformed json":   {`{"email":`, 400, "VALIDATION_ERROR"},
		"missing password": {`{"email":"` + f.email + `"}`, 400, "VALIDATION_ERROR"},
		"blank email":      {`{"email":"  ","password":"x"}`, 400, "VALIDATION_ERROR"},
		"body too large":   {`{"email":"` + f.email + `","password":"` + strings.Repeat("a", 5000) + `"}`, 400, "VALIDATION_ERROR"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(tc.body)))
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

			if tc.wantCode == "" {
				var body struct {
					Token string `json:"token"`
					User  struct {
						ID          string   `json:"id"`
						TenantID    string   `json:"tenantId"`
						DisplayName string   `json:"displayName"`
						Email       string   `json:"email"`
						Roles       []string `json:"roles"`
					} `json:"user"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.NotEmpty(t, body.Token)
				assert.Equal(t, f.userID.String(), body.User.ID)
				assert.Equal(t, f.tenantID.String(), body.User.TenantID)
				assert.Equal(t, f.email, body.User.Email)
				assert.Equal(t, []string{"Admin"}, body.User.Roles)
				return
			}
			var e httpx.Envelope
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &e))
			assert.Equal(t, tc.wantCode, e.ErrorCode)
			assert.NotEmpty(t, e.Message)
			assert.NotEmpty(t, e.RequestID)
		})
	}
}
