package config

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Host      string
	Endpoints []EndpointConfig
	Scenarios map[string]Scenario
}

type EndpointConfig struct {
	Path      string
	Method    string
	Scenarios []string
}

type mainConfig struct {
	Host      string              `yaml:"host"`
	Scenarios map[string]Scenario `yaml:"scenarios"`
	Env       map[string]string   `yaml:"env"`
}

type Scenario struct {
	Params   map[string]string `yaml:"params"`
	Query    map[string]string `yaml:"query"`
	Headers  map[string]string `yaml:"headers"`
	Request  BodyRequest       `yaml:"request"`
	Response Response          `yaml:"response"`
}

type Response struct {
	Status *int `yaml:"status,omitempty"`
	Body   Body `yaml:"body,omitempty"`
}

type StrictString string

type BodyRequest struct {
	Json *map[string]any `yaml:"json,omitempty"`
	Text *StrictString   `yaml:"text,omitempty"`
}

type Body struct {
	Contains []string          `yaml:"contains,omitempty"`
	Json     *map[string]JNode `yaml:"json,omitempty"`
	Text     *StrictString     `yaml:"text,omitempty"`
	Value    any               `yaml:"-"`
}

type JNode struct {
	Value    any                `yaml:"value,omitempty"`
	Type     *string            `yaml:"type,omitempty"`
	Min      *int               `yaml:"min,omitempty"`
	Max      *int               `yaml:"max,omitempty"`
	Contains []string           `yaml:"contains,omitempty"`
	Object   map[string]JNode   `yaml:"object,omitempty"`
	List     []map[string]JNode `yaml:"list,omitempty"`
	ListEach map[string]JNode   `yaml:"-"`
}

type Endpoint struct {
	Summary        string         `yaml:"summary"`
	Responses      map[string]any `yaml:"responses"`
	FunctionalTest struct {
		Scenarios []string `yaml:"scenarios"`
	} `yaml:"v-functional-test"`
}

type OpenAPI struct {
	OpenAPI   string                         `yaml:"openapi"`
	Info      map[string]any                 `yaml:"info"`
	Servers   []map[string]any               `yaml:"servers"`
	Scenarios map[string]Scenario            `yaml:"scenarios"`
	Path      map[string]map[string]Endpoint `yaml:"paths"`
}

func Parse(configPath, openAPIPath string) (*Config, error) {
	cfg := Config{
		Endpoints: []EndpointConfig{},
		Scenarios: make(map[string]Scenario),
	}

	data, err := extractData(configPath)
	if err != nil {
		return nil, err
	}
	var mc mainConfig
	if err := decodeStrictYAML(data, &mc); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}
	cfg.Host = strings.TrimRight(mc.Host, "/")
	for k, sce := range mc.Scenarios {
		cfg.Scenarios[k] = sce
	}

	if openAPIPath != "" {
		oData, err := extractData(openAPIPath)
		if err != nil {
			return nil, err
		}

		var openapi OpenAPI
		if err := decodeStrictYAML(oData, &openapi); err != nil {
			return nil, fmt.Errorf("failed to parse openapi yaml: %w", err)
		}
		for k, sce := range openapi.Scenarios {
			cfg.Scenarios[k] = sce
		}

		for path, val := range openapi.Path {
			for method, endpoint := range val {
				if len(endpoint.FunctionalTest.Scenarios) == 0 {
					continue
				}
				cfg.Endpoints = append(cfg.Endpoints, EndpointConfig{
					Path:      path,
					Method:    strings.ToUpper(method),
					Scenarios: endpoint.FunctionalTest.Scenarios,
				})
			}
		}
	}

	cfg.Host = resolveString(cfg.Host, mc.Env)
	for name, s := range cfg.Scenarios {
		s.addRequiredFields("", s.Response.Body.Json)
		s.resolveScenarios(mc.Env)
		cfg.Scenarios[name] = s
	}
	return &cfg, nil
}

func (s *Scenario) addRequiredFields(prefix string, body *map[string]JNode) {
	if body == nil {
		return
	}
	for key, value := range *body {
		if prefix == "" && key == "list" {
			continue
		}
		if prefix == "" && key == "object" && len(value.Object) > 0 {
			s.addRequiredFields("", &value.Object)
			continue
		}

		path := key
		if prefix != "" {
			path = fmt.Sprintf("%s.%s", prefix, key)
		}
		if len(value.Object) > 0 {
			s.addRequiredFields(path, &value.Object)
		}

		for i, item := range value.List {
			indexPath := fmt.Sprintf("%s[%d]", path, i)
			s.addRequiredFields(indexPath, &item)
		}
		if len(value.ListEach) > 0 {
			s.addRequiredFields(fmt.Sprintf("%s[0]", path), &value.ListEach)
		}
		isUnique := true
		for _, value := range s.Response.Body.Contains {
			if value == path {
				isUnique = false
			}
		}
		if len(value.Object) == 0 && len(value.List) == 0 && len(value.ListEach) == 0 && isUnique {
			s.Response.Body.Contains = append(s.Response.Body.Contains, path)
		}
	}
}

func (s *Scenario) resolveScenarios(envMap map[string]string) {
	for k, v := range s.Params {
		s.Params[k] = resolveString(v, envMap)
	}
	for k, v := range s.Query {
		s.Query[k] = resolveString(v, envMap)
	}
	for k, v := range s.Headers {
		s.Headers[k] = resolveString(v, envMap)
	}
	if s.Request.Json != nil {
		jsonBody := resolveJSONValue(*s.Request.Json, envMap).(map[string]any)
		s.Request.Json = &jsonBody
	}
	if s.Request.Text != nil {
		resolved := StrictString(resolveString(string(*s.Request.Text), envMap))
		s.Request.Text = &resolved
	}

	if s.Response.Body.Json != nil {
		s.handleJSON(*s.Response.Body.Json, envMap)
	}
	if s.Response.Body.Text != nil {
		resolved := StrictString(resolveString(string(*s.Response.Body.Text), envMap))
		s.Response.Body.Text = &resolved
	}
	if str, ok := s.Response.Body.Value.(string); ok {
		s.Response.Body.Value = resolveString(str, envMap)
	}
}

func (s *Scenario) handleJSON(jsonBody map[string]JNode, envMap map[string]string) {
	for key, val := range jsonBody {
		if str, ok := val.Value.(string); ok {
			val.Value = resolveString(str, envMap)
		}

		if _, ok := val.Value.(map[string]any); ok {
			fmt.Printf("⚠️  Warning: scenario field %q has embedded object in 'value'. Use 'object' instead.\n", key)
		}

		if _, ok := val.Value.([]map[string]any); ok {
			fmt.Printf("⚠️  Warning: scenario field %q has embedded object in 'value'. Use 'list' instead.\n", key)
		}

		if s.Response.Body.Json == nil {
			s.Response.Body.Json = &map[string]JNode{}
		}

		jsonMap := *s.Response.Body.Json
		jsonMap[key] = val
		s.Response.Body.Json = &jsonMap

		if len(val.Object) > 0 {
			s.handleJSON(val.Object, envMap)
		}

		for _, item := range val.List {
			s.handleJSON(item, envMap)
		}
		if len(val.ListEach) > 0 {
			s.handleJSON(val.ListEach, envMap)
		}
	}
}

func resolveString(val string, envMap map[string]string) string {
	var envVarRegex = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}|\$([A-Z0-9_]+)`)
	return envVarRegex.ReplaceAllStringFunc(val, func(match string) string {
		groups := envVarRegex.FindStringSubmatch(match)
		key := groups[1]
		if key == "" {
			key = groups[2]
		}

		if v, ok := envMap[key]; ok {
			return v
		}
		if v := os.Getenv(key); v != "" {
			return v
		}
		fmt.Printf("⚠️  Warning: env var %s not found, replacing with empty string\n", key)
		return ""
	})
}

func resolveJSONValue(value any, envMap map[string]string) any {
	switch typed := value.(type) {
	case string:
		return resolveString(typed, envMap)
	case map[string]any:
		for key, item := range typed {
			typed[key] = resolveJSONValue(item, envMap)
		}
		return typed
	case []any:
		for i, item := range typed {
			typed[i] = resolveJSONValue(item, envMap)
		}
		return typed
	default:
		return value
	}
}

func extractData(path string) ([]byte, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		resp, err := http.Get(path)
		if err != nil {
			return nil, fmt.Errorf("error fetching remote config: %w", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading remote config: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config: %w", err)
	}
	return data, err
}

func decodeStrictYAML(data []byte, out any) error {
	return yaml.NewDecoder(bytes.NewReader(data), yaml.Strict()).Decode(out)
}

func (s *StrictString) UnmarshalYAML(data []byte) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string, got %T", value)
	}

	*s = StrictString(str)
	return nil
}

func (b *Body) UnmarshalYAML(data []byte) error {
	var scalar any
	if err := yaml.Unmarshal(data, &scalar); err != nil {
		return err
	}
	if _, ok := scalar.(map[string]any); !ok {
		b.Value = scalar
		return nil
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	var result Body
	var schemaKeys map[string]JNode

	for key, value := range raw {
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return err
		}

		switch key {
		case "contains":
			if err := yaml.Unmarshal(encoded, &result.Contains); err != nil {
				return err
			}
		case "text":
			var text StrictString
			if err := yaml.Unmarshal(encoded, &text); err != nil {
				return err
			}
			result.Text = &text
		case "json":
			normalized, err := decodeBodySchema(encoded)
			if err != nil {
				return err
			}
			schemaKeys = normalized
		case "object":
			normalized, err := decodeBodyObject(encoded)
			if err != nil {
				return err
			}
			schemaKeys = normalized
		case "list":
			normalized, err := decodeBodyList(encoded)
			if err != nil {
				return err
			}
			schemaKeys = normalized
		default:
			if schemaKeys == nil {
				schemaKeys = map[string]JNode{}
			}

			var node JNode
			if err := yaml.Unmarshal(encoded, &node); err != nil {
				return err
			}
			schemaKeys[key] = node
		}
	}

	if len(schemaKeys) > 0 {
		result.Json = &schemaKeys
	}

	*b = result
	return nil
}

// extend UnmarshalYAML to supprot types
func (n *JNode) UnmarshalYAML(data []byte) error {
	type rawJNode struct {
		Value    any              `yaml:"value,omitempty"`
		Type     *string          `yaml:"type,omitempty"`
		Min      *int             `yaml:"min,omitempty"`
		Max      *int             `yaml:"max,omitempty"`
		Contains []string         `yaml:"contains,omitempty"`
		Object   map[string]JNode `yaml:"object,omitempty"`
		List     any              `yaml:"list,omitempty"`
	}

	var structured rawJNode
	if err := yaml.Unmarshal(data, &structured); err == nil {
		if structured.Value == nil &&
			structured.Type == nil &&
			structured.Min == nil &&
			structured.Max == nil &&
			len(structured.Contains) == 0 &&
			len(structured.Object) == 0 &&
			structured.List == nil {
			var implicitObject map[string]JNode
			if err := yaml.Unmarshal(data, &implicitObject); err == nil && len(implicitObject) > 0 {
				*n = JNode{Object: implicitObject}
				return nil
			}
		}

		n.Value = structured.Value
		n.Type = structured.Type
		n.Min = structured.Min
		n.Max = structured.Max
		n.Contains = structured.Contains
		n.Object = structured.Object
		n.List = nil
		n.ListEach = nil

		if structured.List == nil {
			return nil
		}

		listData, err := yaml.Marshal(structured.List)
		if err != nil {
			return err
		}

		var listEntries []map[string]JNode
		if err := yaml.Unmarshal(listData, &listEntries); err == nil {
			n.List = listEntries
			return nil
		}

		var listSchema struct {
			Object map[string]JNode `yaml:"object"`
		}
		if err := yaml.Unmarshal(listData, &listSchema); err == nil && len(listSchema.Object) > 0 {
			n.ListEach = listSchema.Object
			return nil
		}

		var implicitListObject map[string]JNode
		if err := yaml.Unmarshal(listData, &implicitListObject); err == nil && len(implicitListObject) > 0 {
			n.ListEach = implicitListObject
			return nil
		}

		return fmt.Errorf("invalid list schema")
	}

	var scalar any
	if err := yaml.Unmarshal(data, &scalar); err != nil {
		return err
	}

	if _, ok := scalar.(map[string]any); ok {
		return fmt.Errorf("invalid json node schema")
	}

	*n = JNode{Value: scalar}
	return nil
}

func IsRootObjectNode(body *map[string]JNode) (map[string]JNode, bool) {
	if body == nil {
		return nil, false
	}

	jsonBody := *body
	if len(jsonBody) != 1 {
		return nil, false
	}

	root, ok := jsonBody["object"]
	if !ok || len(root.Object) == 0 {
		return nil, false
	}

	return root.Object, true
}

func IsRootListNode(body *map[string]JNode) (JNode, bool) {
	if body == nil {
		return JNode{}, false
	}

	jsonBody := *body
	if len(jsonBody) != 1 {
		return JNode{}, false
	}

	root, ok := jsonBody["list"]
	if !ok {
		return JNode{}, false
	}

	if len(root.List) == 0 && len(root.ListEach) == 0 {
		return JNode{}, false
	}

	return root, true
}

func decodeBodySchema(data []byte) (map[string]JNode, error) {
	var normalized map[string]JNode
	if err := yaml.Unmarshal(data, &normalized); err != nil {
		return nil, err
	}
	if root, ok := normalized["object"]; ok && len(normalized) == 1 && len(root.Object) > 0 {
		return root.Object, nil
	}
	return normalized, nil
}

func decodeBodyObject(data []byte) (map[string]JNode, error) {
	var objectSchema map[string]JNode
	if err := yaml.Unmarshal(data, &objectSchema); err != nil {
		return nil, err
	}
	return objectSchema, nil
}

func decodeBodyList(data []byte) (map[string]JNode, error) {
	var listValue any
	if err := yaml.Unmarshal(data, &listValue); err != nil {
		return nil, err
	}

	encoded, err := yaml.Marshal(listValue)
	if err != nil {
		return nil, err
	}

	var node JNode
	var listEntries []map[string]JNode
	if err := yaml.Unmarshal(encoded, &listEntries); err == nil {
		node.List = listEntries
		return map[string]JNode{"list": node}, nil
	}

	var listSchema struct {
		Object map[string]JNode `yaml:"object"`
	}
	if err := yaml.Unmarshal(encoded, &listSchema); err == nil && len(listSchema.Object) > 0 {
		node.ListEach = listSchema.Object
		return map[string]JNode{"list": node}, nil
	}

	var implicitListObject map[string]JNode
	if err := yaml.Unmarshal(encoded, &implicitListObject); err == nil && len(implicitListObject) > 0 {
		node.ListEach = implicitListObject
		return map[string]JNode{"list": node}, nil
	}

	return nil, fmt.Errorf("invalid list schema")
}
