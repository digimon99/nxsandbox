package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/digimon99/nxsandbox/internal/auth"
)

type AuthHandlers struct {
	authSvc      *auth.Service
	cookieDomain string
}

func NewAuthHandlers(authSvc *auth.Service, cookieDomain string) *AuthHandlers {
	return &AuthHandlers{authSvc: authSvc, cookieDomain: cookieDomain}
}

type signinRequest struct {
	Email string `json:"email"`
}

type verifyRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

func (h *AuthHandlers) SignIn(w http.ResponseWriter, r *http.Request) {
	var req signinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := h.authSvc.RequestOTP(r.Context(), req.Email); err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			Error(w, http.StatusTooManyRequests, "RATE_LIMITED", err.Error())
			return
		}
		Error(w, http.StatusBadRequest, "AUTH_ERROR", err.Error())
		return
	}
	Success(w, map[string]interface{}{
		"message": "verification code sent",
	})
}

func (h *AuthHandlers) Verify(w http.ResponseWriter, r *http.Request) {
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	token, refresh, user, err := h.authSvc.VerifyOTP(r.Context(), req.Email, req.OTP)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidOTP) {
			Error(w, http.StatusUnauthorized, "INVALID_OTP", err.Error())
			return
		}
		Error(w, http.StatusUnauthorized, "AUTH_ERROR", err.Error())
		return
	}

	h.setRefreshCookie(w, refresh)
	expires := time.Now().Add(time.Duration(h.authSvc.AccessTokenTTLSeconds()) * time.Second).Unix()
	Success(w, map[string]interface{}{
		"auth_token": token,
		"expires_at": expires,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		Error(w, http.StatusUnauthorized, "MISSING_REFRESH_TOKEN", "missing refresh token")
		return
	}

	token, user, err := h.authSvc.RefreshSession(r.Context(), cookie.Value)
	if err != nil {
		Error(w, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "invalid or expired refresh token")
		return
	}

	expires := time.Now().Add(time.Duration(h.authSvc.AccessTokenTTLSeconds()) * time.Second).Unix()
	Success(w, map[string]interface{}{
		"auth_token": token,
		"expires_at": expires,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"role":  user.Role,
		},
	})
}

func (h *AuthHandlers) SignOff(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = h.authSvc.RevokeSession(r.Context(), cookie.Value)
	}
	h.clearRefreshCookie(w)
	Success(w, map[string]interface{}{"message": "signed off"})
}

func (h *AuthHandlers) setRefreshCookie(w http.ResponseWriter, refresh string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		Domain:   strings.TrimSpace(h.cookieDomain),
		MaxAge:   h.authSvc.RefreshTokenTTLSeconds(),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandlers) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		Domain:   strings.TrimSpace(h.cookieDomain),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
