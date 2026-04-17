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
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return nil
	}

	printResults(results)

	return nil
}

func printResults(results []executor.Result) {
	for _, result := range results {
		fmt.Printf("\n%s %s\n", result.Method, result.Endpoint)
		for _, scenario := range result.Scenarios {
			if scenario.Success {
				fmt.Printf("  %s %s (%dms) - %s\n", ui.SymbolPass(), scenario.Name, scenario.Millisecond, scenario.Message)
				continue
			}
			fmt.Printf("  %s %s - %s\n", ui.SymbolFail(), scenario.Name, scenario.Message)
		}
	}
}
