package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketReceivesScanEvents(t *testing.T) {
	srv, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	ts := httptest.NewServer(h)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", cookie.String())

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		body := ""
		if resp != nil {
			body = resp.Status
		}
		t.Fatalf("ws dial: %v (%s)", err, body)
	}
	defer conn.Close()

	// Wait for the handler goroutine to register the client before broadcasting,
	// otherwise the scan_started event races the registration.
	deadline := time.Now().Add(2 * time.Second)
	for srv.Hub().ClientCount() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("ws client never registered")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Trigger a scan over REST; expect scan lifecycle events over WS.
	w := doReq(h, http.MethodPost, "/api/scan", nil, cookie)
	if w.Code != http.StatusAccepted {
		t.Fatalf("scan trigger: want 202, got %d", w.Code)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := readEventTypes(t, conn, 2)
	if !got["scan_started"] {
		t.Errorf("did not receive scan_started; got %v", got)
	}
}

func TestWebSocketRejectsUnauthenticated(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	completeSetup(t, st)

	ts := httptest.NewServer(h)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected unauthenticated ws dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		t.Fatalf("want 401 on upgrade, got %d", code)
	}
}

func readEventTypes(t *testing.T, conn *websocket.Conn, max int) map[string]bool {
	t.Helper()
	got := make(map[string]bool)
	for i := 0; i < max; i++ {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var ev Event
		if err := json.Unmarshal(data, &ev); err != nil {
			continue
		}
		got[ev.Event] = true
	}
	return got
}
