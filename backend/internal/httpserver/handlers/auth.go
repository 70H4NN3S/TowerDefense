// Package handlers contains HTTP handler structs, one per resource.
// Each handler is thin: decode → validate → call service → respond.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/johannesniedens/towerdefense/internal/auth"
	"github.com/johannesniedens/towerdefense/internal/httpserver/middleware"
	"github.com/johannesniedens/towerdefense/internal/httpserver/respond"
)

// AuthHandler wires auth-related routes onto a ServeMux.
type AuthHandler struct {
	svc     *auth.Service
	limiter *middleware.IPLimiter
}

// NewAuthHandler constructs an AuthHandler.
// The limiter is applied to /register and /login to throttle brute-force
// attempts. Use NewIPLimiter(10, time.Minute) for production.
func NewAuthHandler(svc *auth.Service, limiter *middleware.IPLimiter) *AuthHandler {
	return &AuthHandler{svc: svc, limiter: limiter}
}

// Register wires routes onto mux.
func (h *AuthHandler) Register(mux *http.ServeMux) {
	limited := middleware.RateLimit(h.limiter)

	mux.Handle("POST /v1/auth/register", limited(http.HandlerFunc(h.handleRegister)))
	mux.Handle("POST /v1/auth/login", limited(http.HandlerFunc(h.handleLogin)))
	mux.Handle("POST /v1/auth/refresh", limited(http.HandlerFunc(h.handleRefresh)))
}

// registerRequest is the JSON body for POST /v1/auth/register.
type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// loginRequest is the JSON body for POST /v1/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// refreshRequest is the JSON body for POST /v1/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// tokenResponse is the JSON shape returned after successful auth operations.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

func (h *AuthHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeBody(r, &req); err != nil {
		http.Error(w, `{"error":{"code":"invalid_body","message":"Request body is not valid JSON."}}`, http.StatusBadRequest)
		return
	}

	pair, err := h.svc.Register(r.Context(), auth.RegisterInput{
		Email:    req.Email,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusCreated, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    int(time.Hour.Seconds()),
	})
}

func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeBody(r, &req); err != nil {
		http.Error(w, `{"error":{"code":"invalid_body","message":"Request body is not valid JSON."}}`, http.StatusBadRequest)
		return
	}

	pair, err := h.svc.Login(r.Context(), auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    int(time.Hour.Seconds()),
	})
}

func (h *AuthHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeBody(r, &req); err != nil {
		http.Error(w, `{"error":{"code":"invalid_body","message":"Request body is not valid JSON."}}`, http.StatusBadRequest)
		return
	}

	pair, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	respond.JSON(w, http.StatusOK, tokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    int(time.Hour.Seconds()),
	})
}

// decodeBody decodes a JSON request body into dst, disallowing unknown fields.
func decodeBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
