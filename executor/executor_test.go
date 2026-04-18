package executor

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/volcapi/config"
)

func TestBuildRequestURL(t *testing.T) {
	s := config.Scenario{
		Params: map[string]string{"id": "42"},
		Query:  map[string]string{"sort": "desc", "page": "1"},
	}

	got := buildRequestURL("https://api.example.com/users/{id}", s)
	if got != "https://api.example.com/users/42?page=1&sort=desc" && got != "https://api.example.com/users/42?sort=desc&page=1" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestReplacePathParamsKeepsUnknownParam(t *testing.T) {
	got := replacePathParams("/users/{id}", map[string]string{})
	if got != "/users/{id}" {
		t.Fatalf("expected unresolved placeholder to remain, got %s", got)
	}
}

func TestRunCollectsScenarioResults(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	status := 200
	cfg := &config.Config{
		Host: ts.URL,
		Endpoints: []config.EndpointConfig{
			{Path: "/ping", Method: http.MethodGet, Scenarios: []string{"good", "missing"}},
		},
		Scenarios: map[string]config.Scenario{
			"good": {
				Response: config.Response{Status: &status},
			},
		},
	}

	results, err := Run(cfg)
	if len(results) != 1 {
		t.Fatalf("expected one endpoint result, got %d", len(results))
	}
	if len(results[0].Scenarios) != 2 {
		t.Fatalf("expected two scenario results, got %d", len(results[0].Scenarios))
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !results[0].Scenarios[0].Success {
		t.Fatal("expected first scenario to pass")
	}
	if results[0].Scenarios[0].RequestURL != ts.URL+"/ping" {
		t.Fatalf("expected first scenario request url %q, got %q", ts.URL+"/ping", results[0].Scenarios[0].RequestURL)
	}
	if results[0].Scenarios[1].Success {
		t.Fatal("expected second scenario to fail")
	}
	if results[0].Scenarios[1].RequestURL != ts.URL+"/ping" {
		t.Fatalf("expected missing scenario request url %q, got %q", ts.URL+"/ping", results[0].Scenarios[1].RequestURL)
	}
}

func TestRunStoresResolvedScenarioRequestURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	status := 200
	cfg := &config.Config{
		Host: ts.URL,
		Endpoints: []config.EndpointConfig{
			{Path: "/users/{id}", Method: http.MethodGet, Scenarios: []string{"good"}},
		},
		Scenarios: map[string]config.Scenario{
			"good": {
				Params:   map[string]string{"id": "42"},
				Query:    map[string]string{"page": "1"},
				Response: config.Response{Status: &status},
			},
		},
	}

	results, err := Run(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := results[0].Scenarios[0].RequestURL
	wantA := ts.URL + "/users/42?page=1"
	if got != wantA {
		t.Fatalf("expected resolved request url %q, got %q", wantA, got)
	}
}

func TestRunFunctionalReturnsMilliseconds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	status := 200
	ms, err := runFunctional(server.URL, http.MethodGet, "timing", config.Scenario{
		Response: config.Response{Status: &status},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ms < 0 || ms > 1000 {
		t.Fatalf("expected a millisecond value, got %d", ms)
	}
}
