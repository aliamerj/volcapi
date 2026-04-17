package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/volcapi/config"
	"github.com/volcapi/ui"
)

var (
	cGreen = color.New(color.FgHiGreen)
	cRed   = color.New(color.FgHiRed)
	cGray  = color.New(color.FgHiBlack)
)

func runFunctional(endpoint, method, scenarioName string, scenario config.Scenario) error {
	label := fmt.Sprintf("%s [%s %s]", scenarioName, method, endpoint)
	spin := ui.ShowSpinner(label)

	bodyBytes, err := requestBody(scenario)
	if err != nil {
		spin.Stop()
		fmt.Printf("   %s %s  request body build failed\n", ui.SymbolFail(), cRed.Sprintf(scenarioName))
		return err
	}

	req, err := http.NewRequest(method, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		spin.Stop()
		fmt.Printf("   %s %s  request build failed\n", ui.SymbolFail(), cRed.Sprintf(scenarioName))
		return err
	}
	for key, value := range scenario.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	spin.Stop()
	if err != nil {
		fmt.Printf("   %s %s  %s\n", ui.SymbolFail(), cRed.Sprintf(scenarioName), cRed.Sprintf(err.Error()))
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if scenario.Response.Status != nil && *scenario.Response.Status != resp.StatusCode {
		err := fmt.Errorf("status mismatch (expected %d, got %d)", *scenario.Response.Status, resp.StatusCode)
		fmt.Printf("   %s %s  %s  %s\n", ui.SymbolFail(), cRed.Sprintf(scenarioName), cRed.Sprintf(err.Error()), cGray.Sprintf("(%v)", elapsed.Truncate(time.Millisecond)))
		return err
	}

	if err := validateResponseBody(resp, respBody, scenario.Response); err != nil {
		fmt.Printf("   %s %s  %s  %s\n", ui.SymbolFail(), cRed.Sprintf(scenarioName), cRed.Sprintf(err.Error()), cGray.Sprintf("(%v)", elapsed.Truncate(time.Millisecond)))
		return err
	}

	fmt.Printf("   %s %s  %s\n", ui.SymbolPass(), cGreen.Sprintf(scenarioName), cGray.Sprintf("(%v)", elapsed.Truncate(time.Millisecond)))
	return nil
}

func requestBody(scenario config.Scenario) ([]byte, error) {
	if scenario.Request.Json != nil {
		return json.Marshal(scenario.Request.Json)
	}
	if scenario.Request.Text != nil {
		return []byte(*scenario.Request.Text), nil
	}
	return []byte(""), nil
}

func validateResponseBody(resp *http.Response, respBody []byte, expect config.Response) error {
	if expect.Body.Text != nil {
		if !strings.Contains(string(respBody), *expect.Body.Text) {
			return fmt.Errorf("expected %q in response body, but not found", *expect.Body.Text)
		}
	}

	for _, contentType := range resp.Header["Content-Type"] {
		if strings.Contains(contentType, "application/json") {
			return validateExpectations(respBody, expect)
		}
	}

	return nil
}

func validateExpectations(respBody []byte, expect config.Response) error {
	var actualBody map[string]any
	if err := json.Unmarshal(respBody, &actualBody); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	for _, path := range expect.Body.Contains {
		val, ok := getByPath(actualBody, path)
		if !ok {
			return fmt.Errorf("expected %s to exist, but it does NOT", path)
		}
		if val == nil {
			return fmt.Errorf("expected %s to exist, but it is NULL", path)
		}
	}

	for key, value := range actualBody {
		if expect.Body.Json != nil {
			jsonBody := *expect.Body.Json
			if expected, ok := jsonBody[key]; ok {
				if err := validateJNode(value, key, expected); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateJNode(actual any, key string, expected config.JNode) error {
	switch realValue := actual.(type) {
	case string:
		expectedValue, match := expected.Value.(string)
		if !match && expected.Value != nil {
			return fmt.Errorf("invalid type: expected %s to be string, got %T", key, expected.Value)
		}

		if expected.Type != nil && *expected.Type != "string" {
			return fmt.Errorf("invalid type expected %s to be string", key)
		}
		if expectedValue != "" && expectedValue != realValue {
			return fmt.Errorf("expected %s to be '%s', got '%s'", key, expectedValue, realValue)
		}
		if expected.Min != nil && len(realValue) < *expected.Min {
			return fmt.Errorf("expected %s to have string length >= %v", key, *expected.Min)
		}
		if expected.Max != nil && len(realValue) > *expected.Max {
			return fmt.Errorf("expected %s to have string length <= %v", key, *expected.Max)
		}
	case float64:
		if exp, ok := expected.Value.(float64); ok && exp != realValue {
			return fmt.Errorf("expected %s to be %v, got %v", key, exp, realValue)
		}
	case bool:
		if expected.Value != nil {
			if exp, ok := expected.Value.(bool); ok && exp != realValue {
				return fmt.Errorf("expected %s to be %v, got %v", key, exp, realValue)
			}
		}
	case map[string]any:
		return validateObject(realValue, expected.Object)
	case []any:
		return validateList(realValue, expected.List)
	default:
		return fmt.Errorf("unsupported type: %T", realValue)
	}
	return nil
}

func validateObject(actual map[string]any, expected map[string]config.JNode) error {
	for k, expNode := range expected {
		actVal, ok := actual[k]
		if !ok {
			return fmt.Errorf("missing key %s", k)
		}
		if err := validateJNode(actVal, k, expNode); err != nil {
			return fmt.Errorf("%s: %v", k, err)
		}
	}
	return nil
}

func validateList(actual []any, expected []map[string]config.JNode) error {
	if len(expected) > 0 && len(actual) != len(expected) {
		return fmt.Errorf("array length mismatch (expected %d, got %d)", len(expected), len(actual))
	}
	for i, act := range actual {
		if len(expected) > 0 {
			if err := validateObject(asMap(act), expected[i]); err != nil {
				return fmt.Errorf("index %d: %v", i, err)
			}
		}
	}
	return nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

type pathToken struct {
	key   string
	index *int
}

var (
	reBracket = regexp.MustCompile(`^([a-zA-Z0-9_-]+)\[(\d+)\]$`)
)

func getByPath(data any, path string) (any, bool) {
	tokens := parsePath(path)
	current := data

	for _, tok := range tokens {
		switch node := current.(type) {
		case map[string]any:
			val, ok := node[tok.key]
			if !ok {
				return nil, false
			}
			current = val
			if tok.index != nil {
				arr, ok := current.([]any)
				if !ok || *tok.index >= len(arr) {
					return nil, false
				}
				current = arr[*tok.index]
			}
		case []any:
			if tok.index == nil || *tok.index >= len(node) {
				return nil, false
			}
			current = node[*tok.index]
		default:
			return nil, false
		}
	}

	return current, true
}

func parsePath(path string) []pathToken {
	parts := strings.Split(path, ".")
	var tokens []pathToken
	for _, p := range parts {
		if m := reBracket.FindStringSubmatch(p); m != nil {
			idx, _ := strconv.Atoi(m[2])
			tokens = append(tokens, pathToken{key: m[1], index: &idx})
		} else {
			tokens = append(tokens, pathToken{key: p})
		}
	}
	return tokens
}
