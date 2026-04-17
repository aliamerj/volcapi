package config

import (
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

type BodyRequest struct {
	Json *map[string]any `yaml:"json,omitempty"`
	Text *string         `yaml:"text,omitempty"`
}

type Body struct {
	Contains []string          `yaml:"contains,omitempty"`
	Json     *map[string]JNode `yaml:"json,omitempty"`
	Text     *string           `yaml:"text,omitempty"`
}

type JNode struct {
	Value    any                `yaml:"value,omitempty"`
	Type     *string            `yaml:"type,omitempty"`
	Min      *int               `yaml:"min,omitempty"`
	Max      *int               `yaml:"max,omitempty"`
	Contains []string           `yaml:"contains,omitempty"`
	Object   map[string]JNode   `yaml:"object,omitempty"`
	List     []map[string]JNode `yaml:"list,omitempty"`
}

type Endpoint struct {
	FunctionalTest struct {
		Scenarios []string `yaml:"scenarios"`
	} `yaml:"v-functional-test"`
}

type OpenAPI struct {
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
	if err := yaml.Unmarshal(data, &mc); err != nil {
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
		if err := yaml.Unmarshal(oData, &openapi); err != nil {
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
		isUnique := true
		for _, value := range s.Response.Body.Contains {
			if value == path {
				isUnique = false
			}
		}
		if len(value.Object) == 0 && len(value.List) == 0 && isUnique {
			s.Response.Body.Contains = append(s.Response.Body.Contains, path)
		}
	}
}

func (s *Scenario) resolveScenarios(envMap map[string]string) {
	for k, v := range s.Headers {
		s.Headers[k] = resolveString(v, envMap)
	}
	if s.Request.Json != nil {
		jsonBody := *s.Request.Json
		for k, v := range jsonBody {
			if str, ok := v.(string); ok {
				jsonBody[k] = resolveString(str, envMap)
				s.Request.Json = &jsonBody
			}
		}
	}

	if s.Response.Body.Json != nil {
		s.handleJSON(*s.Response.Body.Json, envMap)
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
	}
}

func resolveString(val string, envMap map[string]string) string {
	vars := strings.Split(val, " ")
	var envVarRegex = regexp.MustCompile(`^\$[A-Z0-9_]+$`)
	for i, word := range vars {
		if !envVarRegex.MatchString(word) {
			continue
		}
		key := strings.TrimPrefix(word, "$")

		if v, ok := envMap[key]; ok {
			vars[i] = v
			continue
		}
		if v := os.Getenv(key); v != "" {
			vars[i] = v
			continue
		}
		fmt.Printf("⚠️  Warning: env var %s not found, replacing with empty string\n", key)
		vars[i] = ""
	}

	return strings.Join(vars, " ")
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

