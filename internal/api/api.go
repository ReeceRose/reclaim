package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"reclaim/internal/config"
	"reclaim/internal/store"
)

// ScanTrigger is the slice of the scanner the API drives. Kept as an interface
// so handlers are testable without a real fsnotify-backed scanner.
type ScanTrigger interface {
	Scan(ctx context.Context, trigger string, force bool) (*store.ScanRun, error)
}

// Deps are the wired dependencies the API needs. main.go builds this; tests
// build a partial one with fakes.
type Deps struct {
	Store       *store.Store
	Scanner     ScanTrigger
	Live        *config.Live
	MoviesPath  string
	TVPath      string
	DisableAuth bool
}

// Server holds injected dependencies and wires routes. Calling Handler()
// returns a standard http.Handler so main.go never imports echo directly —
// swap the framework here only.
type Server struct {
	store       *store.Store
	scanner     ScanTrigger
	live        *config.Live
	auth        AuthStore
	moviesPath  string
	tvPath      string
	disableAuth bool

	hub          *Hub
	loginLimiter *rateLimiter
}

func New(d Deps) *Server {
	s := &Server{
		store:        d.Store,
		scanner:      d.Scanner,
		live:         d.Live,
		moviesPath:   d.MoviesPath,
		tvPath:       d.TVPath,
		disableAuth:  d.DisableAuth,
		hub:          NewHub(),
		loginLimiter: newRateLimiter(),
	}
	if d.Store != nil {
		s.auth = d.Store.Settings
	}
	return s
}

// Hub exposes the WebSocket broadcaster so the scanner/worker wiring in main.go
// can push progress events.
func (s *Server) Hub() *Hub { return s.hub }

// Handler builds the Echo instance and returns it as http.Handler.
func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(echo.WrapMiddleware(AuthMiddleware(s.auth, s.disableAuth)))

	e.GET("/healthz", s.healthz)

	api := e.Group("/api")

	// Auth (unprotected by the middleware allow-list).
	api.POST("/setup", s.handleSetup)
	api.POST("/login", s.handleLogin)
	api.POST("/logout", s.handleLogout)
	api.GET("/session", s.handleSession)

	// Read side.
	api.GET("/stats", s.handleStats)
	api.GET("/candidates", s.handleCandidates)
	api.GET("/files/:id", s.handleFileDetail)
	api.GET("/dry-run", s.handleDryRun)

	// Scanning.
	api.POST("/scan", s.handleScan)
	api.POST("/scan/full", s.handleFullScan)

	// Profiles (CRUD §11).
	api.GET("/profiles", s.handleListProfiles)
	api.POST("/profiles", s.handleCreateProfile)
	api.PUT("/profiles/:id", s.handleUpdateProfile)
	api.DELETE("/profiles/:id", s.handleDeleteProfile)

	// Jobs.
	api.POST("/jobs", s.handleCreateJobs)
	api.GET("/jobs", s.handleListJobs)
	api.POST("/jobs/:id/cancel", s.handleCancelJob)

	// Settings.
	api.GET("/settings", s.handleGetSettings)
	api.PUT("/settings", s.handlePutSettings)
	api.PUT("/settings/credentials", s.handleChangeCredentials)

	// Live progress.
	api.GET("/ws", s.handleWS)

	return e
}

func (s *Server) healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
