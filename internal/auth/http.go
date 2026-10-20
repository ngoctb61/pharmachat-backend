package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ngoctb61/pharmachat-backend/internal/platform/httpx"
)

// maxLoginBody bounds the request body; a login is two short strings.
const maxLoginBody = 4 << 10

var errInvalidLoginBody = &httpx.Error{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: "Vui long nhap email va mat khau"}

// Handler serves the /api/auth routes.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler returns the auth HTTP handler.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login handles POST /api/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxLoginBody)).Decode(&req); err != nil ||
		strings.TrimSpace(req.Email) == "" || req.Password == "" {
		httpx.WriteError(w, r, h.log, errInvalidLoginBody)
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, h.log, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
