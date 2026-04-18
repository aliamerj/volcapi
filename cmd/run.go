package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/volcapi/config"
	"github.com/volcapi/executor"
	"github.com/volcapi/ui"
)

// runCmd represents the run command
var (
	openapiPath string

	runCmd = &cobra.Command{
		Use:   "run <config-path> [flags]",
		Short: "Run functional API tests from a config file",
		Long: `Execute VolcAPI tests defined in a YAML config.

The config file can be loaded from a local path or a remote URL.
Optionally, provide an OpenAPI spec for validation.

Examples:
  volcapi run volcapi_local.yml -o openapi.yml
  volcapi run https://example.com/tests/volcapi_local.yml -o openapi.yml
`,
		Args: cobra.ExactArgs(1),
		RunE: runRun,
	}
)

func init() {
	runCmd.Flags().StringVarP(&openapiPath, "openapi", "o", "", "Path to an OpenAPI specification (YAML/JSON)")
}

func runRun(cmd *cobra.Command, args []string) error {
	if args[0] == "" {
		return fmt.Errorf("You must provide a config path (local file or URL)")
	}
	conf, err := config.Parse(args[0], openapiPath)
	if err != nil {
		return err
	}

	results, err := executor.Run(conf)
	printResults(results)
	if err != nil {
		fmt.Println(err.Error())
	}

	return nil
}

func printResults(results []executor.Result) {
	totalEndpoints := len(results)
	totalScenarios := 0
	passedScenarios := 0

	fmt.Printf("\n%s\n", ui.Section("VolcAPI Results"))

	for _, result := range results {
		endpointPassed := true
		fmt.Printf("\n%s\n", ui.EndpointHeader(result.Method, result.Endpoint))
		for _, scenario := range result.Scenarios {
			totalScenarios++
			if scenario.Success {
				passedScenarios++
				fmt.Printf("  %s %-24s %s %s\n", ui.SymbolPass(), scenario.Name, ui.Muted(fmt.Sprintf("%4dms", scenario.Millisecond)), ui.Muted(scenario.Message))
				if scenario.RequestURL != "" && scenario.RequestURL != result.Endpoint {
					fmt.Printf("    %s %s\n", ui.SymbolInfo(), ui.Muted(scenario.RequestURL))
				}
				continue
			}
			endpointPassed = false
			fmt.Printf("  %s %-24s %s\n", ui.SymbolFail(), scenario.Name, resultFailureMessage(scenario.Message))
			if scenario.RequestURL != "" && scenario.RequestURL != result.Endpoint {
				fmt.Printf("    %s %s\n", ui.SymbolInfo(), ui.Muted(scenario.RequestURL))
			}
		}

		if endpointPassed {
			fmt.Printf("  %s %s\n", ui.SymbolInfo(), ui.Muted("endpoint passed"))
		} else {
			fmt.Printf("  %s %s\n", ui.SymbolWarn(), ui.Muted("endpoint has failing scenarios"))
		}
	}

	failedScenarios := totalScenarios - passedScenarios
	fmt.Printf(
		"\n%s\n  %s %s\n  %s %s\n  %s %s\n\n",
		ui.Section("Summary"),
		ui.SymbolInfo(), ui.Muted(fmt.Sprintf("endpoints: %d", totalEndpoints)),
		ui.SymbolPass(), ui.Muted(fmt.Sprintf("passed: %d", passedScenarios)),
		ui.SymbolFail(), ui.Muted(fmt.Sprintf("failed: %d", failedScenarios)),
	)
}

func resultFailureMessage(message string) string {
	return ui.Accent(message)
}
