package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStore struct {
	setupDone bool
	validUser string
	validPass string
	secret    []byte
}

func (f *fakeStore) IsSetupComplete() bool          { return f.setupDone }
func (f *fakeStore) ValidateLogin(u, p string) bool { return u == f.validUser && p == f.validPass }
func (f *fakeStore) SessionSecret() []byte          { return f.secret }

var testSecret = []byte("test-secret-32-bytes-long-enough!")

func newSetupDoneStore() *fakeStore {
	return &fakeStore{setupDone: true, validUser: "admin", validPass: "pass", secret: testSecret}
}

func cookiedRequest(username string) *http.Request {
	r := httptest.NewRequest("GET", "/api/stats", nil)
	IssueSession(httptest.NewRecorder(), username, testSecret, false)
	// re-issue into the request cookie jar manually
	w := httptest.NewRecorder()
	IssueSession(w, username, testSecret, false)
	for _, c := range w.Result().Cookies() {
		r.AddCookie(c)
	}
	return r
}

func okHandler(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

func applyMiddleware(store AuthStore, disableAuth bool, req *http.Request) *httptest.ResponseRecorder {
	h := AuthMiddleware(store, disableAuth)(http.HandlerFunc(okHandler))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestAuth_setupIncomplete_redirectsToSetup(t *testing.T) {
	store := &fakeStore{setupDone: false, secret: testSecret}
	r := httptest.NewRequest("GET", "/api/stats", nil)
	w := applyMiddleware(store, false, r)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/setup" {
		t.Fatalf("want redirect to /setup, got %q", loc)
	}
}

func TestAuth_setupComplete_missingCookie_api401(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/stats", nil)
	w := applyMiddleware(newSetupDoneStore(), false, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_setupComplete_missingCookie_spaRedirectsToLogin(t *testing.T) {
	r := httptest.NewRequest("GET", "/candidates", nil)
	w := applyMiddleware(newSetupDoneStore(), false, r)
	if w.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("want redirect to /login, got %q", loc)
	}
}

func TestAuth_validCookie_passes(t *testing.T) {
	r := cookiedRequest("admin")
	w := applyMiddleware(newSetupDoneStore(), false, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestAuth_invalidCookie_api401(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/stats", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "garbage.badsig"})
	w := applyMiddleware(newSetupDoneStore(), false, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestAuth_disableAuth_alwaysPasses(t *testing.T) {
	store := &fakeStore{setupDone: false, secret: testSecret}
	r := httptest.NewRequest("GET", "/api/stats", nil)
	w := applyMiddleware(store, true, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestAuth_unprotectedRoutes_alwaysPass(t *testing.T) {
	store := &fakeStore{setupDone: false, secret: testSecret}
	for _, path := range []string{"/healthz", "/api/setup", "/api/login", "/api/logout"} {
		r := httptest.NewRequest("GET", path, nil)
		w := applyMiddleware(store, false, r)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s: want 200, got %d", path, w.Code)
		}
	}
}
