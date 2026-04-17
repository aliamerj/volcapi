package executor

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/volcapi/config"
)

func Run(cfg *config.Config) error {
	if len(cfg.Endpoints) == 0 {
		return fmt.Errorf("no testable endpoints were found; provide an OpenAPI file with v-functional-test scenarios")
	}

	failed := 0
	total := 0

	for _, endpoint := range cfg.Endpoints {
		for _, scenarioName := range endpoint.Scenarios {
			total++
			scenario, ok := cfg.Scenarios[scenarioName]
			if !ok {
				fmt.Printf("   ✖ %s  scenario not found\n", scenarioName)
				failed++
				continue
			}

			requestURL := buildRequestURL(cfg.Host+endpoint.Path, scenario)
			if err := runFunctional(requestURL, endpoint.Method, scenarioName, scenario); err != nil {
				failed++
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("test run completed with %d/%d failed scenarios", failed, total)
	}

	fmt.Printf("\n✔ All scenarios passed (%d/%d)\n", total-failed, total)
	return nil
}

func buildRequestURL(rawPath string, scenario config.Scenario) string {
	resolvedPath := replacePathParams(rawPath, scenario.Params)
	if len(scenario.Query) == 0 {
		return resolvedPath
	}

	parsed, err := url.Parse(resolvedPath)
	if err != nil {
		return resolvedPath
	}

	q := parsed.Query()
	for key, value := range scenario.Query {
		q.Set(key, value)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func replacePathParams(path string, params map[string]string) string {
	re := regexp.MustCompile(`\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(path, func(match string) string {
		key := strings.Trim(match, "{}")
		if val, ok := params[key]; ok {
			return val
		}
		return match
	})
}

