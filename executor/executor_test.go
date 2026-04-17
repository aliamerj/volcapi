package executor

import (
	"net/http"
	"net/http/httptest"
	"testing"

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
	if err == nil {
		t.Fatal("expected error because one scenario is missing")
	}
	if len(results) != 1 {
		t.Fatalf("expected one endpoint result, got %d", len(results))
	}
	if len(results[0].Scenarios) != 2 {
		t.Fatalf("expected two scenario results, got %d", len(results[0].Scenarios))
	}
	if !results[0].Scenarios[0].Success {
		t.Fatal("expected first scenario to pass")
	}
	if results[0].Scenarios[1].Success {
		t.Fatal("expected second scenario to fail")
	}
}

