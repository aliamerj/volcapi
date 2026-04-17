package executor

import (
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
