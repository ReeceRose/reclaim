package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// Event is the typed envelope for every WS message. Push-only: all commands stay
// on REST (§P5), which keeps the reconnection story simple.
type Event struct {
	Event string `json:"event"`
	Data  any    `json:"data"`
}

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = (wsPongWait * 9) / 10
	wsSendBuffer = 32
)

// Hub is the broadcast fan-out for live job + scan progress. A dropped-slow
// client is disconnected rather than allowed to block the broadcaster.
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*wsClient]struct{})}
}

// Broadcast marshals an event once and delivers it to every connected client.
func (h *Hub) Broadcast(event string, data any) {
	payload, err := json.Marshal(Event{Event: event, Data: data})
	if err != nil {
		slog.Error("ws: marshal event", "event", event, "err", err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for cl := range h.clients {
		select {
		case cl.send <- payload:
		default:
			// Client is too slow; drop the connection rather than block.
			close(cl.send)
			delete(h.clients, cl)
		}
	}
}

// ClientCount reports the number of connected clients (used in tests).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) register(cl *wsClient) {
	h.mu.Lock()
	h.clients[cl] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(cl *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[cl]; ok {
		delete(h.clients, cl)
		close(cl.send)
	}
	h.mu.Unlock()
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Same-origin SPA served by this binary; no cross-origin check needed on a
	// single-user LAN tool. The session cookie already gates the upgrade.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWS upgrades to a WebSocket after validating the session cookie. The
// auth middleware already 401s /api/* without a cookie, but per §P5 the upgrade
// handler validates the cookie itself so the handshake can never slip through.
func (s *Server) handleWS(c echo.Context) error {
	r := c.Request()
	if !s.disableAuth {
		secret := s.store.Settings.SessionSecret()
		if !s.store.Settings.IsSetupComplete() || !hasValidSession(r, secret) {
			return c.JSON(http.StatusUnauthorized, errorBody("unauthorized"))
		}
	}

	conn, err := wsUpgrader.Upgrade(c.Response(), r, nil)
	if err != nil {
		return err // Upgrade already wrote the error response.
	}

	cl := &wsClient{conn: conn, send: make(chan []byte, wsSendBuffer)}
	s.hub.register(cl)

	go cl.writePump()
	cl.readPump(s.hub)
	return nil
}

// readPump drains incoming frames (we ignore client messages) and handles
// pongs + connection close, unregistering on exit.
func (c *wsClient) readPump(h *Hub) {
	defer func() {
		h.unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump delivers broadcasts and periodic pings.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				// Hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
