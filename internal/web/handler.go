package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/digimon99/nxsandbox/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	authSvc      *auth.Service
	cookieDomain string
	templates    *template.Template
}

func NewHandler(authSvc *auth.Service, cookieDomain string) (*Handler, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{authSvc: authSvc, cookieDomain: strings.TrimSpace(cookieDomain), templates: tmpl}, nil
}

func (h *Handler) Mount(r chi.Router) {
	if sub, err := fs.Sub(staticFS, "static"); err == nil {
		r.Handle("/static/*", http.StripPrefix("/static", http.FileServer(http.FS(sub))))
	}

	r.Get("/", h.home)
	r.Get("/signin", h.signinPage)
	r.Post("/signin", h.signin)
	r.Get("/signin/verify", h.verifyPage)
	r.Post("/signin/verify", h.verify)
	r.Get("/verified", h.verified)
	r.Get("/signoff", h.signoff)
	r.Get("/dashboard", h.dashboardPlaceholder)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	h.render(w, "home.html", nil)
}

func (h *Handler) signinPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "signin.html", map[string]interface{}{
		"Error": humanError(r.URL.Query().Get("error")),
	})
}

func (h *Handler) signin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signin?error=bad_request", http.StatusSeeOther)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	if err := h.authSvc.RequestOTP(r.Context(), email); err != nil {
		query := url.Values{}
		query.Set("error", "otp_failed")
		http.Redirect(w, r, "/signin?"+query.Encode(), http.StatusSeeOther)
		return
	}
	query := url.Values{}
	query.Set("email", email)
	http.Redirect(w, r, "/signin/verify?"+query.Encode(), http.StatusSeeOther)
}

func (h *Handler) verifyPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, "verify.html", map[string]interface{}{
		"Error": humanError(r.URL.Query().Get("error")),
		"Email": strings.TrimSpace(r.URL.Query().Get("email")),
	})
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/signin/verify?error=bad_request", http.StatusSeeOther)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	otp := strings.TrimSpace(r.FormValue("otp"))
	token, refresh, user, err := h.authSvc.VerifyOTP(r.Context(), email, otp)
	if err != nil {
		query := url.Values{}
		query.Set("email", email)
		query.Set("error", "invalid_otp")
		http.Redirect(w, r, "/signin/verify?"+query.Encode(), http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		Domain:   h.cookieDomain,
		MaxAge:   h.authSvc.RefreshTokenTTLSeconds(),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	expires := time.Now().Add(time.Duration(h.authSvc.AccessTokenTTLSeconds()) * time.Second).Unix()
	userPayload, _ := json.Marshal(map[string]interface{}{
		"id":    user.ID,
		"email": user.Email,
		"role":  user.Role,
	})
	userB64 := base64.StdEncoding.EncodeToString(userPayload)
	query := url.Values{}
	query.Set("token", token)
	query.Set("expires", strconv.FormatInt(expires, 10))
	query.Set("user", userB64)
	http.Redirect(w, r, "/verified?"+query.Encode(), http.StatusSeeOther)
}

func (h *Handler) verified(w http.ResponseWriter, r *http.Request) {
	h.render(w, "verified.html", nil)
}

func (h *Handler) signoff(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		_ = h.authSvc.RevokeSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		Domain:   h.cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	h.render(w, "signoff.html", nil)
}

func (h *Handler) dashboardPlaceholder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<html><body><h1>Dashboard placeholder</h1></body></html>"))
}

func (h *Handler) render(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf := bytes.NewBuffer(nil)
	if err := h.templates.ExecuteTemplate(buf, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(buf.Bytes())
}

func humanError(code string) string {
	switch strings.TrimSpace(code) {
	case "invalid_otp":
		return "Invalid or expired code."
	case "otp_failed":
		return "Could not send verification code."
	case "bad_request":
		return "Invalid request."
	default:
		return ""
	}
}
