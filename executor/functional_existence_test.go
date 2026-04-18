package executor

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/volcapi/config"
)

func TestValidateExpectationsBareTrueMeansFieldExists(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{name: "string", body: map[string]any{"id": "abc"}},
		{name: "number", body: map[string]any{"id": 42}},
		{name: "bool", body: map[string]any{"id": false}},
		{name: "null", body: map[string]any{"id": nil}},
	}

	expect := config.Response{
		Body: config.Body{
			Contains: []string{"id"},
			Json: &map[string]config.JNode{
				"id": {Value: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respBody, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}

			if err := validateExpectations(respBody, expect); err != nil {
				t.Fatalf("expected existence check to pass, got %v", err)
			}
		})
	}
}

func TestValidateExpectationsBareTrueFailsWhenFieldMissing(t *testing.T) {
	respBody, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	expect := config.Response{
		Body: config.Body{
			Contains: []string{"id"},
			Json: &map[string]config.JNode{
				"id": {Value: true},
			},
		},
	}

	if err := validateExpectations(respBody, expect); err == nil {
		t.Fatal("expected missing field to fail")
	}
}

func TestValidateExpectationsRootObjectListEachSchemaPasses(t *testing.T) {
	respBody, err := json.Marshal(map[string]any{
		"data": []any{
			map[string]any{
				"id":     1,
				"name":   "Ali",
				"email":  "ali@example.com",
				"status": "active",
			},
			map[string]any{
				"id":     "2",
				"name":   "Sara",
				"email":  "sara@example.com",
				"status": true,
			},
		},
		"meta": map[string]any{"total": 2},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	expect := config.Response{
		Body: config.Body{
			Contains: []string{"data", "meta"},
			Json: &map[string]config.JNode{
				"object": {
					Object: map[string]config.JNode{
						"data": {
							ListEach: map[string]config.JNode{
								"id":     {Value: true},
								"name":   {Value: true},
								"email":  {Value: true},
								"status": {Value: true},
							},
						},
					},
				},
			},
		},
	}

	if err := validateExpectations(respBody, expect); err != nil {
		t.Fatalf("expected root object list schema to pass, got %v", err)
	}
}

func TestValidateResponseBodyEmptyJSONPassesWithoutBodyExpectations(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	expect := config.Response{}

	if err := validateResponseBody(resp, []byte(""), expect); err != nil {
		t.Fatalf("expected empty JSON body to pass without body expectations, got %v", err)
	}
}

func TestValidateResponseBodyEmptyJSONFailsWithBodyExpectations(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	expect := config.Response{
		Body: config.Body{
			Contains: []string{"id"},
		},
	}

	err := validateResponseBody(resp, []byte(""), expect)
	if err == nil {
		t.Fatal("expected empty JSON body with assertions to fail")
	}
	if err.Error() != "expected JSON response body, but the body is empty" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateExpectationsRootListSchemaPasses(t *testing.T) {
	respBody, err := json.Marshal([]any{
		map[string]any{
			"id":     1,
			"name":   "Ali",
			"status": "active",
		},
		map[string]any{
			"id":     2,
			"name":   "Sara",
			"status": "inactive",
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	expect := config.Response{
		Body: config.Body{
			Json: &map[string]config.JNode{
				"list": {
					ListEach: map[string]config.JNode{
						"id":     {Value: true},
						"name":   {Value: true},
						"status": {Value: true},
					},
				},
			},
		},
	}

	if err := validateExpectations(respBody, expect); err != nil {
		t.Fatalf("expected root list schema to pass, got %v", err)
	}
}

func TestValidateExpectationsRootObjectFailsForStringJSON(t *testing.T) {
	respBody, err := json.Marshal("hello")
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	expect := config.Response{
		Body: config.Body{
			Json: &map[string]config.JNode{
				"message": {Value: true},
			},
		},
	}

	err = validateExpectations(respBody, expect)
	if err == nil {
		t.Fatal("expected top-level string JSON to fail against object schema")
	}
}

func TestValidateResponseBodyTextOnlyIgnoresBrokenJSONContentType(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	text := "Dashboard"
	expect := config.Response{
		Body: config.Body{
			Text: (*config.StrictString)(&text),
		},
	}

	if err := validateResponseBody(resp, []byte("<title>Dashboard</title>"), expect); err != nil {
		t.Fatalf("expected text-only validation to pass, got %v", err)
	}
}

func TestValidateResponseBodyTextOnlyFailsWhenSubstringMissing(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	text := "Dashboard"
	expect := config.Response{
		Body: config.Body{
			Text: (*config.StrictString)(&text),
		},
	}

	err := validateResponseBody(resp, []byte("plain text"), expect)
	if err == nil {
		t.Fatal("expected text-only validation to fail when substring is missing")
	}
}

func TestValidateResponseBodyExactScalarStringPasses(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
		},
	}

	expect := config.Response{
		Body: config.Body{
			Value: "hello world",
		},
	}

	if err := validateResponseBody(resp, []byte("hello world\n"), expect); err != nil {
		t.Fatalf("expected exact scalar string body to pass, got %v", err)
	}
}

func TestValidateResponseBodyExactScalarBoolPasses(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	expect := config.Response{
		Body: config.Body{
			Value: true,
		},
	}

	if err := validateResponseBody(resp, []byte("true"), expect); err != nil {
		t.Fatalf("expected exact scalar bool body to pass, got %v", err)
	}
}

func TestValidateResponseBodyExactScalarMismatchFails(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	expect := config.Response{
		Body: config.Body{
			Value: 123,
		},
	}

	err := validateResponseBody(resp, []byte("456"), expect)
	if err == nil {
		t.Fatal("expected exact scalar body mismatch to fail")
	}
}

func TestValidateResponseBodyJSONAssertionsFailForNonJSONContentType(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
		},
	}

	expect := config.Response{
		Body: config.Body{
			Contains: []string{"message"},
		},
	}

	err := validateResponseBody(resp, []byte(`{"message":"hello"}`), expect)
	if err == nil {
		t.Fatal("expected JSON assertions on non-JSON content type to fail")
	}
	if err.Error() != `expected JSON response content-type, got "text/plain"` {
		t.Fatalf("unexpected error: %v", err)
	}
}
