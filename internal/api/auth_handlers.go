package api

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"reclaim/internal/store"
)

const minPasswordLen = 8

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r credentialsRequest) validate() error {
	if strings.TrimSpace(r.Username) == "" {
		return errors.New("username must not be empty")
	}
	if len(r.Password) < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// handleSetup performs first-run setup once. Subsequent calls return 409.
func (s *Server) handleSetup(c echo.Context) error {
	var req credentialsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := req.validate(); err != nil {
		return badRequest(c, err.Error())
	}

	err := s.store.Settings.CompleteSetup(c.Request().Context(), req.Username, req.Password)
	if errors.Is(err, store.ErrSetupAlreadyComplete) {
		return c.JSON(http.StatusConflict, errorBody("setup already complete"))
	}
	if err != nil {
		return serverError(c, err)
	}

	// Log the user straight in so the SPA can proceed to the app after setup.
	IssueSession(c.Response(), req.Username, s.store.Settings.SessionSecret(), isSecureRequest(c.Request()))
	return c.JSON(http.StatusOK, map[string]any{"username": req.Username})
}

// handleLogin validates credentials and issues a signed session cookie.
func (s *Server) handleLogin(c echo.Context) error {
	ip := clientIP(c.Request())
	if !s.loginLimiter.allow(ip) {
		return c.JSON(http.StatusTooManyRequests, errorBody("too many login attempts, slow down"))
	}

	var req credentialsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if !s.store.Settings.ValidateLogin(req.Username, req.Password) {
		return c.JSON(http.StatusUnauthorized, errorBody("invalid username or password"))
	}

	s.loginLimiter.reset(ip)
	IssueSession(c.Response(), req.Username, s.store.Settings.SessionSecret(), isSecureRequest(c.Request()))
	return c.JSON(http.StatusOK, map[string]any{"username": req.Username})
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(c echo.Context) error {
	ClearSession(c.Response())
	return c.NoContent(http.StatusNoContent)
}

// handleSession reports setup + auth state so the SPA can route to setup, login,
// or the app on load. It is reachable unauthenticated by design.
func (s *Server) handleSession(c echo.Context) error {
	resp := map[string]any{
		"setup_complete": s.store.Settings.IsSetupComplete(),
		"authenticated":  false,
		"username":       nil,
	}
	if s.disableAuth {
		resp["authenticated"] = true
		return c.JSON(http.StatusOK, resp)
	}
	if user, ok := sessionUsername(c.Request(), s.store.Settings.SessionSecret()); ok {
		resp["authenticated"] = true
		resp["username"] = user
	}
	return c.JSON(http.StatusOK, resp)
}

// handleChangeCredentials re-bcrypts and stores new credentials. Never returns
// the hash. Behind the session gate.
func (s *Server) handleChangeCredentials(c echo.Context) error {
	var req credentialsRequest
	if err := c.Bind(&req); err != nil {
		return badRequest(c, "invalid JSON body")
	}
	if err := req.validate(); err != nil {
		return badRequest(c, err.Error())
	}
	err := s.store.Settings.ChangeCredentials(c.Request().Context(), req.Username, req.Password)
	if errors.Is(err, store.ErrSetupNotComplete) {
		return badRequest(c, "setup not complete")
	}
	if err != nil {
		return serverError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"username": req.Username})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if i := strings.IndexByte(fwd, ','); i >= 0 {
			return strings.TrimSpace(fwd[:i])
		}
		return strings.TrimSpace(fwd)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		return host[:i]
	}
	return host
}

// rateLimiter is a small per-key fixed-window counter. It exists only to slow
// password brute force on the login endpoint (§4.2), not as a robust limiter.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*window
	max      int
	window   time.Duration
}

type window struct {
	count int
	start time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string]*window),
		max:      10,
		window:   time.Minute,
	}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w, ok := l.attempts[key]
	if !ok || now.Sub(w.start) > l.window {
		l.attempts[key] = &window{count: 1, start: now}
		return true
	}
	if w.count >= l.max {
		return false
	}
	w.count++
	return true
}

func (l *rateLimiter) reset(key string) {
	l.mu.Lock()
	delete(l.attempts, key)
	l.mu.Unlock()
}
