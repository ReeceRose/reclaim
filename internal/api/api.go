package api

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"reclaim/internal/config"
	"reclaim/internal/store"
)

// ScanTrigger is the slice of the scanner the API drives. Kept as an interface
// so handlers are testable without a real fsnotify-backed scanner.
type ScanTrigger interface {
	StartScan(ctx context.Context, trigger string, force bool) error
}

// JobCanceller is the slice of the worker the API drives to cancel a running
// encode. Cancel returns true if the job was actively running and its
// ffmpeg was killed; false means the worker isn't running it, so the handler
// falls back to flipping the DB state for a merely-queued job.
type JobCanceller interface {
	Cancel(jobID int64) bool
}

// MetadataFetcher is the background TMDB metadata fetcher. Trigger enqueues a
// re-fetch run; RefreshKey force-refreshes a single entry.
type MetadataFetcher interface {
	Trigger()
	RefreshKey(ctx context.Context, key, mediaType string) error
}

// Deps are the wired dependencies the API needs. main.go builds this; tests
// build a partial one with fakes.
type Deps struct {
	Store           *store.Store
	Scanner         ScanTrigger
	Backfill        BackfillCoordinator
	Live            *config.Live
	MoviesPath      string
	TVPath          string
	TMDBKey         string
	DisableAuth     bool
	MetadataFetcher MetadataFetcher

	// StaticFS is the embedded frontend (Next.js static export). When nil, no
	// static routes are mounted — handy for API-only tests.
	StaticFS fs.FS
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
	tmdbKey     string
	disableAuth bool
	staticFS    fs.FS

	hub          *Hub
	loginLimiter *rateLimiter
	canceller    JobCanceller
	metaFetcher  MetadataFetcher
	backfill     BackfillCoordinator
}

func New(d Deps) *Server {
	s := &Server{
		store:        d.Store,
		scanner:      d.Scanner,
		live:         d.Live,
		moviesPath:   d.MoviesPath,
		tvPath:       d.TVPath,
		tmdbKey:      d.TMDBKey,
		disableAuth:  d.DisableAuth,
		staticFS:     d.StaticFS,
		hub:          NewHub(),
		loginLimiter: newRateLimiter(),
		metaFetcher:  d.MetadataFetcher,
		backfill:     d.Backfill,
	}
	if d.Store != nil {
		s.auth = d.Store.Settings
	}
	return s
}

// Hub exposes the WebSocket broadcaster so the scanner/worker wiring in main.go
// can push progress events.
func (s *Server) Hub() *Hub { return s.hub }

// SetCanceller wires the worker in after construction (the worker needs the
// server's Hub, so the two are built in sequence and joined here).
func (s *Server) SetCanceller(c JobCanceller) { s.canceller = c }

// Handler builds the Echo instance and returns it as http.Handler.
func (s *Server) Handler() http.Handler {
	e := echo.New()

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c *echo.Context) bool {
			// Skip noisy static asset requests from the embedded Next.js frontend.
			return strings.HasPrefix(c.Request().URL.Path, "/_next/")
		},
		LogStatus:   true,
		LogMethod:   true,
		LogURI:      true,
		LogLatency:  true,
		LogRemoteIP: true,
		HandleError: true,
		LogValuesFunc: func(_ *echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []any{
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
				"latency", v.Latency.String(),
				"remote_ip", v.RemoteIP,
			}
			if v.Error != nil {
				attrs = append(attrs, "err", v.Error)
				slog.Error("http request", attrs...)
			} else {
				slog.Info("http request", attrs...)
			}
			return nil
		},
	}))
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
	api.GET("/files", s.handleFiles)
	api.GET("/files/grouped/episodes", s.handleGroupedFileEpisodes)
	api.GET("/files/grouped/seasons", s.handleGroupedFileSeasons)
	api.GET("/files/grouped", s.handleGroupedFiles)
	api.GET("/candidates", s.handleCandidates)
	api.GET("/files/:id", s.handleFileDetail)

	// Compatibility ("Direct play").
	api.GET("/compatibility/profiles", s.handleCompatibilityProfiles)
	api.GET("/compatibility/stats", s.handleCompatibilityStats)
	api.GET("/compatibility", s.handleCompatibility)

	// Scanning.
	api.GET("/backfill", s.handleBackfill)
	api.POST("/scan", s.handleScan)
	api.POST("/scan/full", s.handleFullScan)

	// Profiles.
	api.GET("/profiles", s.handleListProfiles)
	api.POST("/profiles", s.handleCreateProfile)
	api.PUT("/profiles/:id", s.handleUpdateProfile)
	api.DELETE("/profiles/:id", s.handleDeleteProfile)

	// Jobs.
	api.POST("/jobs", s.handleCreateJobs)
	api.GET("/jobs", s.handleListJobs)
	api.POST("/jobs/:id/cancel", s.handleCancelJob)
	api.POST("/jobs/:id/force", s.handleForceJob)
	api.DELETE("/jobs/:id", s.handleDeleteJob)

	// Events audit log.
	api.GET("/events", s.handleListEvents)
	api.DELETE("/events", s.handleDeleteAllEvents)
	api.DELETE("/events/:id", s.handleDeleteEvent)

	// Settings.
	api.GET("/settings", s.handleGetSettings)
	api.PUT("/settings", s.handlePutSettings)
	api.PUT("/settings/credentials", s.handleChangeCredentials)

	// Metadata (TMDB).
	api.GET("/metadata", s.handleMetadataGet)
	api.GET("/metadata/search", s.handleMetadataSearch)
	api.PUT("/metadata", s.handleMetadataOverride)
	api.POST("/metadata/refresh", s.handleMetadataRefresh)

	// Live progress.
	api.GET("/ws", s.handleWS)

	// Static SPA (embedded Next.js export). Registered last as a catch-all so
	// it never shadows the API/health routes. Unknown paths fall back to the
	// shell so client-side routes resolve.
	if s.staticFS != nil {
		staticHandler := echo.WrapHandler(newStaticHandler(s.staticFS))
		e.GET("/*", staticHandler)
		e.HEAD("/*", staticHandler)
	}

	return e
}

// newStaticHandler serves embedded static files, falling back to index.html for
// any path that doesn't resolve to a file (SPA client-side routing).
func newStaticHandler(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if upath == "" {
			upath = "index.html"
		}
		if f, err := fsys.Open(upath); err == nil {
			info, err := f.Stat()
			if err == nil && !info.IsDir() {
				defer f.Close()
				serveFSFile(w, r, info, f)
				return
			}
			f.Close()
			// A directory with trailingSlash:true export (e.g. /candidates/)
			// resolves to <dir>/index.html. Serve it before falling back to the
			// root shell, otherwise a hard reload of a sub-route renders the home
			// page instead of the requested page.
			if err == nil && info.IsDir() {
				if idx := path.Join(upath, "index.html"); serveNamedHTML(w, r, fsys, idx) {
					return
				}
			}
		}
		serveNotFound(w, r, fsys)
	})
}

// serveNotFound serves the prebuilt 404 page (Next.js static export) with a 404
// status so unmatched URLs render the styled not-found UI instead of silently
// falling back to the home shell. Older builds without a 404.html degrade to
// the SPA shell.
func serveNotFound(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	f, err := fsys.Open("404.html")
	if err != nil {
		serveIndexHTML(w, r, fsys)
		return
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.IsDir() {
		serveIndexHTML(w, r, fsys)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.Copy(w, f)
}

func serveIndexHTML(w http.ResponseWriter, r *http.Request, fsys fs.FS) {
	if !serveNamedHTML(w, r, fsys, "index.html") {
		http.NotFound(w, r)
	}
}

// serveNamedHTML serves a specific file from the embedded FS. It reports whether
// the file was found and served; callers use the result to decide on fallbacks.
func serveNamedHTML(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	serveFSFile(w, r, info, f)
	return true
}

func serveFSFile(w http.ResponseWriter, r *http.Request, info fs.FileInfo, f fs.File) {
	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, r, info.Name(), info.ModTime(), rs)
		return
	}
	if strings.HasSuffix(info.Name(), ".html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	_, _ = io.Copy(w, f)
}

func (s *Server) healthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) tmdbAPIKey() string {
	return s.tmdbKey
}
