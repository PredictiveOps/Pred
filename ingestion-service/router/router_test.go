package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes_Registered(t *testing.T) {
	r := NewRouter(nil)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/health"},
		{http.MethodPost, "/devices/register"},
		{http.MethodGet, "/devices/1"},
		{http.MethodGet, "/tenants/1/devices"},
		{http.MethodPut, "/devices/1/status"},
		{http.MethodDelete, "/devices/1"},
		{http.MethodGet, "/metrics"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			r.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				t.Errorf("route %s %s not registered (got 404)", tc.method, tc.path)
			}
		})
	}
}

func TestRoutes_UnknownPath(t *testing.T) {
	r := NewRouter(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/no-such-path", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", w.Code)
	}
}

func TestRoutes_WrongMethod(t *testing.T) {
	r := NewRouter(nil)

	cases := []struct {
		wrongMethod string
		path        string
	}{
		// GET /devices/register is intentionally excluded: gin matches it to
		// GET /devices/:device_id (device_id="register") and returns 400.
		{http.MethodPost, "/health"},
		{http.MethodPost, "/devices/1"},
		{http.MethodPost, "/tenants/1/devices"},
		{http.MethodGet, "/devices/1/status"},
	}

	for _, tc := range cases {
		t.Run(tc.wrongMethod+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.wrongMethod, tc.path, nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
				t.Errorf("%s %s: expected 405 or 404, got %d", tc.wrongMethod, tc.path, w.Code)
			}
			if w.Code == http.StatusOK {
				t.Errorf("%s %s: wrong method should not succeed", tc.wrongMethod, tc.path)
			}
		})
	}
}

func TestHealth_Response(t *testing.T) {
	r := NewRouter(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: got %d, want 200", w.Code)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("GET /health: empty response body")
	}
	const want = `"status"`
	for i := 0; i <= len(body)-len(want); i++ {
		if body[i:i+len(want)] == want {
			return
		}
	}
	t.Errorf("GET /health: body %q does not contain %q", body, want)
}

func TestRoutes_Count(t *testing.T) {
	// Verify the routes table hasn't silently grown or shrunk.
	const wantCount = 7
	if len(routes) != wantCount {
		t.Errorf("routes count: got %d, want %d — update this test if you intentionally changed the route table", len(routes), wantCount)
	}
}
