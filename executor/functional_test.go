package executor

import "testing"

func TestGetByPathWithArrayIndex(t *testing.T) {
	payload := map[string]any{
		"users": []any{
			map[string]any{"name": "Alice"},
		},
	}

	v, ok := getByPath(payload, "users[0].name")
	if !ok {
		t.Fatal("expected path to resolve")
	}
	if v != "Alice" {
		t.Fatalf("expected Alice, got %v", v)
	}
}
