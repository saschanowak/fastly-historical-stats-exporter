package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- stringSlice ----------

func TestStringSliceString(t *testing.T) {
	tests := []struct {
		in   stringSlice
		want string
	}{
		{nil, ""},
		{stringSlice{"a"}, "a"},
		{stringSlice{"a", "b", "c"}, "a,b,c"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("stringSlice(%v).String() = %q, want %q", []string(tt.in), got, tt.want)
		}
	}
}

func TestStringSliceSetAppends(t *testing.T) {
	var s stringSlice
	if err := s.Set("first"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := s.Set("second"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if len(s) != 2 || s[0] != "first" || s[1] != "second" {
		t.Errorf("after two Set calls got %v, want [first second]", []string(s))
	}
}

func TestStringSliceSetNeverErrors(t *testing.T) {
	var s stringSlice
	if err := s.Set(""); err != nil {
		t.Errorf("Set(\"\") returned error %v, want nil", err)
	}
}

// ---------- healthzHandler ----------

func TestHealthzReturnsOK(t *testing.T) {
	w := httptest.NewRecorder()
	healthzHandler(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(w.Body.String()); got != "ok" {
		t.Errorf("body = %q, want %q", got, "ok")
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/plain; charset=utf-8")
	}
}

// ---------- rootHandler ----------

func TestRootHandlerServesIndex(t *testing.T) {
	w := httptest.NewRecorder()
	rootHandler(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := w.Body.String()
	for _, want := range []string{"Fastly Historical Stats Exporter", "/metrics", "/healthz"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/html; charset=utf-8")
	}
}

func TestRootHandlerReturns404ForUnknownPaths(t *testing.T) {
	for _, path := range []string{"/unknown", "/foo/bar", "/healthz-extra"} {
		w := httptest.NewRecorder()
		rootHandler(w, httptest.NewRequest(http.MethodGet, path, nil))

		if w.Code != http.StatusNotFound {
			t.Errorf("path %q: status = %d, want %d", path, w.Code, http.StatusNotFound)
		}
	}
}

func TestRootHandlerIncludesProgramVersion(t *testing.T) {
	original := programVersion
	programVersion = "v1.2.3-test"
	defer func() { programVersion = original }()

	w := httptest.NewRecorder()
	rootHandler(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(w.Body.String(), "v1.2.3-test") {
		t.Error("response body does not contain the expected program version")
	}
}
