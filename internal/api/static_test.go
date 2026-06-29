package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func staticFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":            {Data: []byte("<html>home</html>")},
		"404.html":              {Data: []byte("<html>not found</html>")},
		"candidates/index.html": {Data: []byte("<html>candidates</html>")},
		"_next/app.js":          {Data: []byte("console.log(1)")},
	}
}

func serveStatic(path string) *httptest.ResponseRecorder {
	h := newStaticHandler(staticFS())
	r := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestStatic_root_servesIndex(t *testing.T) {
	w := serveStatic("/")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "home") {
		t.Fatalf("want index shell, got %q", w.Body.String())
	}
}

func TestStatic_exportedSubRoute_servesItsIndex(t *testing.T) {
	w := serveStatic("/candidates/")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "candidates") {
		t.Fatalf("want candidates page, got %q", w.Body.String())
	}
}

func TestStatic_unknownPath_serves404Page(t *testing.T) {
	w := serveStatic("/does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not found") {
		t.Fatalf("want 404 page body, got %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want html content-type, got %q", ct)
	}
}

func TestStatic_missing404_fallsBackToShell(t *testing.T) {
	fsys := staticFS()
	delete(fsys, "404.html")
	h := newStaticHandler(fsys)
	r := httptest.NewRequest("GET", "/does-not-exist", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 shell fallback, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "home") {
		t.Fatalf("want index shell fallback, got %q", w.Body.String())
	}
}
