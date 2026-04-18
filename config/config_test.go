package config

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestResolveStringSupportsDollarAndBraces(t *testing.T) {
	t.Setenv("API_KEY", "from-env")

	envMap := map[string]string{
		"TOKEN": "abc123",
	}

	got := resolveString("Bearer $TOKEN and ${API_KEY}", envMap)
	want := "Bearer abc123 and from-env"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveScenariosReplacesNestedRequestValues(t *testing.T) {
	os.Setenv("REGION", "eu")
	defer os.Unsetenv("REGION")

	scenario := Scenario{
		Query: map[string]string{"region": "${REGION}"},
		Request: BodyRequest{
			Json: &map[string]any{
				"user": map[string]any{
					"name": "$NAME",
				},
			},
		},
	}

	scenario.resolveScenarios(map[string]string{"NAME": "ali"})

	if got := scenario.Query["region"]; got != "eu" {
		t.Fatalf("expected resolved query param, got %q", got)
	}
	user := (*scenario.Request.Json)["user"].(map[string]any)
	if got := user["name"]; got != "ali" {
		t.Fatalf("expected resolved nested request value, got %v", got)
	}
}

func TestJNodeUnmarshalAcceptsBooleanShorthand(t *testing.T) {
	var payload struct {
		Body map[string]JNode `yaml:"body"`
	}

	if err := yaml.Unmarshal([]byte("body:\n  id: true\n"), &payload); err != nil {
		t.Fatalf("unmarshal shorthand JNode: %v", err)
	}

	value, ok := payload.Body["id"].Value.(bool)
	if !ok {
		t.Fatalf("expected boolean value, got %T", payload.Body["id"].Value)
	}
	if !value {
		t.Fatal("expected shorthand boolean to be true")
	}
}

func TestJNodeUnmarshalAcceptsListObjectShorthand(t *testing.T) {
	var payload struct {
		Body map[string]JNode `yaml:"body"`
	}

	input := []byte("body:\n  data:\n    list:\n      object:\n        id: true\n        email: true\n")
	if err := yaml.Unmarshal(input, &payload); err != nil {
		t.Fatalf("unmarshal list object shorthand: %v", err)
	}

	if len(payload.Body["data"].ListEach) != 2 {
		t.Fatalf("expected per-item list schema, got %+v", payload.Body["data"].ListEach)
	}
}

func TestJNodeUnmarshalAcceptsImplicitObjectShorthand(t *testing.T) {
	var payload struct {
		Body map[string]JNode `yaml:"body"`
	}

	input := []byte("body:\n  object:\n    data:\n      id: true\n")
	if err := yaml.Unmarshal(input, &payload); err != nil {
		t.Fatalf("unmarshal implicit object shorthand: %v", err)
	}

	root := payload.Body["object"]
	if len(root.Object) != 1 {
		t.Fatalf("expected implicit object fields, got %+v", root.Object)
	}
	if _, ok := root.Object["data"]; !ok {
		t.Fatalf("expected data field inside implicit object, got %+v", root.Object)
	}
}

func TestDecodeStrictYAMLRejectsUnknownFields(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  get_text_body:
    responses:
      status: 200
`), &payload)
	if err == nil {
		t.Fatal("expected unknown field to fail")
	}
	if !strings.Contains(err.Error(), `unknown field "responses"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeStrictYAMLRejectsWrongTextType(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  get_broken_json:
    response:
      status: 200
      body:
        text: true
`), &payload)
	if err == nil {
		t.Fatal("expected wrong text type to fail")
	}
	if !strings.Contains(err.Error(), "cannot unmarshal") {
		if !strings.Contains(err.Error(), "expected string") {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
}

func TestDecodeStrictYAMLAcceptsStringTextType(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  get_text:
    response:
      status: 200
      body:
        text: hello
`), &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeStrictYAMLAcceptsBodyListShorthand(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  search_user:
    response:
      status: 200
      body:
        list:
          id: true
          name: true
          status: true
`), &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := payload.Scenarios["search_user"].Response.Body
	if body.Json == nil {
		t.Fatal("expected parsed body schema")
	}
	if _, ok := IsRootListNode(body.Json); !ok {
		t.Fatalf("expected root list schema, got %+v", *body.Json)
	}
}

func TestDecodeStrictYAMLAcceptsImplicitBodyObjectShorthand(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  get_user:
    response:
      status: 200
      body:
        id: true
        name: true
`), &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := payload.Scenarios["get_user"].Response.Body
	if body.Json == nil {
		t.Fatal("expected parsed body schema")
	}
	if _, ok := (*body.Json)["id"]; !ok {
		t.Fatalf("expected implicit object schema, got %+v", *body.Json)
	}
}

func TestDecodeStrictYAMLAcceptsScalarBodyString(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  get_text_body:
    response:
      status: 200
      body: "hello world"
`), &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := payload.Scenarios["get_text_body"].Response.Body.Value; got != "hello world" {
		t.Fatalf("expected scalar body value, got %#v", got)
	}
}

func TestDecodeStrictYAMLAcceptsScalarBodyBool(t *testing.T) {
	var payload mainConfig

	err := decodeStrictYAML([]byte(`
scenarios:
  get_boolean:
    response:
      status: 200
      body: true
`), &payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := payload.Scenarios["get_boolean"].Response.Body.Value; got != true {
		t.Fatalf("expected boolean scalar body value, got %#v", got)
	}
}
