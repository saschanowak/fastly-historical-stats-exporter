package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/saschanowak/fastly-historical-stats-exporter/pkg/api"
)

// newTestClient creates a Client pointed at baseURL with a short timeout.
func newTestClient(baseURL string) *api.Client {
	return api.NewClient("test-token", api.WithBaseURL(baseURL))
}

// ---------- APIError ----------

func TestAPIErrorWithMessage(t *testing.T) {
	err := &api.APIError{Code: 403, Msg: "forbidden"}
	want := "fastly API responded with 403: forbidden"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIErrorWithoutMessage(t *testing.T) {
	err := &api.APIError{Code: 500}
	want := "fastly API responded with 500"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// ---------- ListServices ----------

func TestListServicesSuccess(t *testing.T) {
	want := []api.Service{
		{ID: "svc1", Name: "Service One", Version: 1},
		{ID: "svc2", Name: "Service Two", Version: 2},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Fastly-Key") != "test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	got, err := newTestClient(srv.URL).ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("ListServices() returned %d services, want %d", len(got), len(want))
	}
	for i, s := range got {
		if s.ID != want[i].ID || s.Name != want[i].Name {
			t.Errorf("service[%d] = %+v, want %+v", i, s, want[i])
		}
	}
}

func TestListServicesFollowsPagination(t *testing.T) {
	page1 := []api.Service{{ID: "svc1", Name: "Service One"}}
	page2 := []api.Service{{ID: "svc2", Name: "Service Two"}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("X-Next-Page", "2")
			json.NewEncoder(w).Encode(page1)
		} else {
			json.NewEncoder(w).Encode(page2)
		}
	}))
	defer srv.Close()

	got, err := newTestClient(srv.URL).ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListServices() returned %d services, want 2", len(got))
	}
	if got[0].ID != "svc1" || got[1].ID != "svc2" {
		t.Errorf("unexpected service IDs: %v, %v", got[0].ID, got[1].ID)
	}
}

func TestListServicesStopsOnNonProgressingNextPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Next-Page", "1") // same page — must not loop
		json.NewEncoder(w).Encode([]api.Service{{ID: "svc1"}})
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 request, got %d (non-progressing next-page should stop pagination)", calls)
	}
}

func TestListServicesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"msg": "invalid token"})
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).ListServices(context.Background())
	if err == nil {
		t.Fatal("ListServices() expected error, got nil")
	}
	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T: %v", err, err)
	}
	if apiErr.Code != http.StatusForbidden {
		t.Errorf("Code = %d, want %d", apiErr.Code, http.StatusForbidden)
	}
	if apiErr.Msg != "invalid token" {
		t.Errorf("Msg = %q, want %q", apiErr.Msg, "invalid token")
	}
}

func TestListServicesInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).ListServices(context.Background())
	if err == nil {
		t.Fatal("ListServices() expected error for invalid JSON, got nil")
	}
}

func TestListServicesContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// hang until the client gives up
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := newTestClient(srv.URL).ListServices(ctx)
	if err == nil {
		t.Fatal("ListServices() expected error on context cancellation, got nil")
	}
}

// ---------- GetStats ----------

func TestGetStatsSuccess(t *testing.T) {
	payload := map[string]any{
		"status": "success",
		"data": []map[string]any{
			{"start_time": 1700000000, "requests": 100.0, "hits": 80.0, "miss": 20.0},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Fastly-Key") != "test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	from := time.Unix(1700000000, 0)
	to := time.Unix(1700000060, 0)
	got, err := newTestClient(srv.URL).GetStats(context.Background(), "svc1", from, to)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetStats() returned %d entries, want 1", len(got))
	}
	if got[0].Requests != 100 {
		t.Errorf("Requests = %v, want 100", got[0].Requests)
	}
	if got[0].Hits != 80 {
		t.Errorf("Hits = %v, want 80", got[0].Hits)
	}
}

func TestGetStatsIncludesTimeRangeInURL(t *testing.T) {
	from := time.Unix(1700000000, 0)
	to := time.Unix(1700000060, 0)
	var capturedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []any{}})
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetStats(context.Background(), "svc1", from, to)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	for _, key := range []string{"from=1700000000", "to=1700000060", "by=minute"} {
		if !containsStr(capturedQuery, key) {
			t.Errorf("query %q missing expected param %q", capturedQuery, key)
		}
	}
}

func TestGetStatsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"msg": "unauthorized"})
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetStats(context.Background(), "svc1", time.Now().Add(-2*time.Minute), time.Now().Add(-time.Minute))
	if err == nil {
		t.Fatal("GetStats() expected error, got nil")
	}
	apiErr, ok := err.(*api.APIError)
	if !ok {
		t.Fatalf("expected *api.APIError, got %T: %v", err, err)
	}
	if apiErr.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", apiErr.Code, http.StatusUnauthorized)
	}
}

func TestGetStatsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).GetStats(context.Background(), "svc1", time.Now().Add(-2*time.Minute), time.Now().Add(-time.Minute))
	if err == nil {
		t.Fatal("GetStats() expected error for invalid JSON, got nil")
	}
}

func TestGetStatsServiceIDIsPathEscaped(t *testing.T) {
	var rawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RawPath holds the encoded form; Path holds the decoded form.
		rawPath = r.URL.RawPath
		if rawPath == "" {
			rawPath = r.URL.Path // no encoding needed (no special chars)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []any{}})
	}))
	defer srv.Close()

	// Service IDs with slashes must be percent-encoded so the path isn't split.
	_, err := newTestClient(srv.URL).GetStats(context.Background(), "svc/with/slash", time.Now().Add(-2*time.Minute), time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if !containsStr(rawPath, "%2F") {
		t.Errorf("expected slash in service ID to be percent-encoded in raw path %q", rawPath)
	}
}

// containsStr is a simple substring check used in place of strings.Contains
// to avoid importing the strings package.
func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
