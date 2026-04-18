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

	"github.com/volcapi/config"
	"github.com/volcapi/ui"
)

func runFunctional(endpoint, method, scenarioName string, scenario config.Scenario) (int, error) {
	label := fmt.Sprintf("%s [%s %s]", scenarioName, method, endpoint)
	spin := ui.ShowSpinner(label)

	bodyBytes, err := requestBody(scenario)
	if err != nil {
		spin.Stop()
		return 0, fmt.Errorf("%s:Failed to build request body: %s", scenarioName, err.Error())
	}

	req, err := http.NewRequest(method, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		spin.Stop()
		return 0, fmt.Errorf("%s:Failed to build request body: %s", scenarioName, err.Error())
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
		return 0, fmt.Errorf("%s: %s", scenarioName, err.Error())
	}

	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if scenario.Response.Status != nil && *scenario.Response.Status != resp.StatusCode {
		return 0, fmt.Errorf("%s: status mismatch (expected %d, got %d)", scenarioName, *scenario.Response.Status, resp.StatusCode)
	}

	if err := validateResponseBody(resp, respBody, scenario.Response); err != nil {
		return 0, fmt.Errorf("%s: %s", scenarioName, err.Error())
	}

	return int(elapsed / time.Millisecond), nil
}

func requestBody(scenario config.Scenario) ([]byte, error) {
	if scenario.Request.Json != nil {
		return json.Marshal(scenario.Request.Json)
	}
	if scenario.Request.Text != nil {
		return []byte(string(*scenario.Request.Text)), nil
	}
	return []byte(""), nil
}

func validateResponseBody(resp *http.Response, respBody []byte, expect config.Response) error {
	isEmptyBody := len(bytes.TrimSpace(respBody)) == 0

	if expect.Body.Value != nil {
		if isEmptyBody {
			return fmt.Errorf("expected %q in response body, but the body is empty", expectedScalarBodyString(expect.Body.Value))
		}
		actual := string(bytes.TrimSpace(respBody))
		expected := expectedScalarBodyString(expect.Body.Value)
		if actual != expected {
			return fmt.Errorf("expected exact response body %q, got %q", expected, actual)
		}
		return nil
	}

	if expect.Body.Text != nil {
		if isEmptyBody {
			return fmt.Errorf("expected %q in response body, but the body is empty", string(*expect.Body.Text))
		}
		if !strings.Contains(string(respBody), string(*expect.Body.Text)) {
			return fmt.Errorf("expected %q in response body, but not found", string(*expect.Body.Text))
		}
	}

	if !expectsJSONValidation(expect) {
		if isEmptyBody && hasBodyExpectations(expect) {
			return fmt.Errorf("expected response body, but the body is empty")
		}
		return nil
	}

	for _, contentType := range resp.Header["Content-Type"] {
		if strings.Contains(contentType, "application/json") {
			if isEmptyBody {
				if expectsJSONValidation(expect) {
					return fmt.Errorf("expected JSON response body, but the body is empty")
				}
				return nil
			}
			return validateExpectations(respBody, expect)
		}
	}

	if expectsJSONValidation(expect) {
		return fmt.Errorf("expected JSON response content-type, got %q", strings.Join(resp.Header["Content-Type"], ", "))
	}

	if isEmptyBody && hasBodyExpectations(expect) {
		return fmt.Errorf("expected response body, but the body is empty")
	}

	return nil
}

func hasBodyExpectations(expect config.Response) bool {
	return expect.Body.Value != nil || expect.Body.Text != nil || len(expect.Body.Contains) > 0 || expect.Body.Json != nil
}

func expectsJSONValidation(expect config.Response) bool {
	return len(expect.Body.Contains) > 0 || expect.Body.Json != nil
}

func expectedScalarBodyString(value any) string {
	return fmt.Sprint(value)
}

func validateExpectations(respBody []byte, expect config.Response) error {
	var actualBody any
	if err := json.Unmarshal(respBody, &actualBody); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	for _, path := range expect.Body.Contains {
		_, ok := getByPath(actualBody, path)
		if !ok {
			return fmt.Errorf("expected %s to exist, but it does NOT", path)
		}
	}

	if rootExpected, ok := config.IsRootListNode(expect.Body.Json); ok {
		actualList, ok := actualBody.([]any)
		if !ok {
			return fmt.Errorf("expected top-level JSON array, got %T", actualBody)
		}
		return validateList(actualList, rootExpected)
	}

	if rootExpected, ok := config.IsRootObjectNode(expect.Body.Json); ok {
		actualObject, ok := actualBody.(map[string]any)
		if !ok {
			return fmt.Errorf("expected top-level JSON object, got %T", actualBody)
		}
		return validateObject(actualObject, rootExpected)
	}

	actualObject, ok := actualBody.(map[string]any)
	if !ok {
		return fmt.Errorf("expected top-level JSON object, got %T", actualBody)
	}

	for key, value := range actualObject {
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
	if isExistenceOnly(expected) {
		return nil
	}

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
		return validateList(realValue, expected)
	default:
		return fmt.Errorf("unsupported type: %T", realValue)
	}
	return nil
}

func isExistenceOnly(expected config.JNode) bool {
	exists, ok := expected.Value.(bool)
	return ok &&
		exists &&
		expected.Type == nil &&
		expected.Min == nil &&
		expected.Max == nil &&
		len(expected.Contains) == 0 &&
		len(expected.Object) == 0 &&
		len(expected.List) == 0 &&
		len(expected.ListEach) == 0
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

func validateList(actual []any, expected config.JNode) error {
	if len(expected.ListEach) > 0 {
		for i, act := range actual {
			if err := validateObject(asMap(act), expected.ListEach); err != nil {
				return fmt.Errorf("index %d: %v", i, err)
			}
		}
		return nil
	}

	if len(expected.List) > 0 && len(actual) != len(expected.List) {
		return fmt.Errorf("array length mismatch (expected %d, got %d)", len(expected.List), len(actual))
	}
	for i, act := range actual {
		if len(expected.List) > 0 {
			if err := validateObject(asMap(act), expected.List[i]); err != nil {
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
