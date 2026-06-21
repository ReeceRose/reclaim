package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Server holds injected dependencies and wires routes.
// Calling Handler() returns a standard http.Handler so main.go
// never imports echo directly — swap the framework here only.
type Server struct {
	auth        AuthStore
	disableAuth bool
}

func New(auth AuthStore, disableAuth bool) *Server {
	return &Server{auth: auth, disableAuth: disableAuth}
}

// Handler builds the Echo instance and returns it as http.Handler.
func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(echo.WrapMiddleware(AuthMiddleware(s.auth, s.disableAuth)))

	e.GET("/healthz", s.healthz)

	return e
}

func (s *Server) healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
