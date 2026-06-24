package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"path"
	"strings"
)

// AuthStore is implemented by the settings repo. Tested here against a fake.
type AuthStore interface {
	IsSetupComplete() bool
	ValidateLogin(username, password string) bool
	SessionSecret() []byte
}

const sessionCookieName = "reclaim_session"

// AuthMiddleware gates all routes except /healthz, /api/setup, /api/login, /api/logout.
// Behavior depends on setup state and DISABLE_AUTH.
func AuthMiddleware(store AuthStore, disableAuth bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if disableAuth {
				next.ServeHTTP(w, r)
				return
			}

			// Always allow: health, auth endpoints, and the static SPA shell +
			// its assets (so the login/setup pages can actually render — they
			// fetch /api/session to decide what to show).
			if isUnprotected(r.URL.Path) || isPublicSPA(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if !store.IsSetupComplete() {
				// Only the setup endpoint is reachable until first-run is done
				http.Redirect(w, r, "/setup", http.StatusFound)
				return
			}

			if !hasValidSession(r, store.SessionSecret()) {
				if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func isUnprotected(p string) bool {
	switch p {
	case "/healthz", "/api/setup", "/api/login", "/api/logout", "/api/session":
		return true
	}
	return false
}

// isPublicSPA reports whether a path is the SPA shell entry (login/setup) or a
// static asset that must load unauthenticated so the shell can render. The app
// is a single-page client that decides setup-vs-login-vs-app from /api/session,
// so these must serve the shell rather than redirect (which would loop).
func isPublicSPA(p string) bool {
	switch p {
	case "/login", "/setup":
		return true
	}
	if strings.HasPrefix(p, "/_next/") {
		return true
	}
	// Top-level static files (favicon.ico, manifest, etc.). API routes and
	// extension-less app routes are excluded so they still gate as before.
	if p != "/" && !strings.HasPrefix(p, "/api/") && path.Ext(p) != "" {
		return true
	}
	return false
}

// IssueSession writes a signed session cookie for the given username.
func IssueSession(w http.ResponseWriter, username string, secret []byte, secure bool) {
	sig := sign(username, secret)
	value := base64.RawURLEncoding.EncodeToString([]byte(username)) + "." + sig
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

// ClearSession removes the session cookie.
func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func hasValidSession(r *http.Request, secret []byte) bool {
	_, ok := sessionUsername(r, secret)
	return ok
}

// sessionUsername validates the session cookie and returns the username it
// carries. ok is false for a missing, malformed, or badly-signed cookie.
func sessionUsername(r *http.Request, secret []byte) (string, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return "", false
	}
	userBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	if !hmac.Equal([]byte(parts[1]), []byte(sign(string(userBytes), secret))) {
		return "", false
	}
	return string(userBytes), true
}

// isSecureRequest reports whether the connection is HTTPS, so the session cookie
// only gets the Secure flag when it can actually be honored. On a plain-HTTP LAN
// the flag is omitted (documented tradeoff).
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func sign(payload string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
