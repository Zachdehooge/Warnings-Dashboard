package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Zachdehooge/warnings-dashboard/internal/fetcher"
	"github.com/Zachdehooge/warnings-dashboard/internal/generator"
	"github.com/spf13/cobra"
)

var (
	outputFile string
	verbose    bool
	interval   int
	watchMode  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "weather-warnings",
		Short: "Fetch and generate weather warnings HTML",
		Long: `Weather Warnings CLI fetches active weather warnings 
from the National Weather Service and generates a static HTML page.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Generate warnings HTML
			err := generateWarningsHTML(cmd)
			if err != nil {
				cmd.PrintErrln(fmt.Errorf("failed to generate warnings: %w", err))
				os.Exit(1)
			}

			// Watch mode
			if watchMode {
				startHTTPServer(cmd)
				runWatchMode(cmd)
			}
		},
	}

	// Flags
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "warnings.html", "Output HTML file path")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().IntVarP(&interval, "interval", "i", 300, "Update interval in seconds (minimum 30)")
	rootCmd.Flags().BoolVar(&watchMode, "watch", false, "Continuously update the warnings HTML")

	// Additional commands
	addListCmd(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func generateWarningsHTML(cmd *cobra.Command) error {
	if verbose {
		cmd.Println("Fetching active weather warnings...")
	}

	warnings, err := fetcher.FetchWarnings()
	if err != nil {
		return fmt.Errorf("failed to fetch warnings: %w", err)
	}

	if verbose {
		cmd.Println(fmt.Sprintf("Generating HTML to %s...", outputFile))
	}

	err = generator.GenerateWarningsHTML(warnings, outputFile)
	if err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	cmd.Println(fmt.Sprintf("Weather warnings saved to %s", outputFile))
	return nil
}

func runWatchMode(cmd *cobra.Command) {
	if interval < 30 {
		interval = 30
	}

	jsonPath := filepath.Join(filepath.Dir(outputFile), "warnings.json")
	generator.StartPoller(jsonPath, 15*time.Second)
	cmd.Println(fmt.Sprintf("Poller started — writing %s every 15s", jsonPath))

	cmd.Println(fmt.Sprintf("Watch mode activated. Updating every %d seconds. Press Ctrl+C to stop.", interval))

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		err := generateWarningsHTML(cmd)
		if err != nil {
			cmd.PrintErrln(fmt.Errorf("update failed: %w", err))
		}
	}
}

func startHTTPServer(cmd *cobra.Command) {
	dir := filepath.Dir(outputFile)
	fs := http.FileServer(http.Dir(dir))

	noCache := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			h.ServeHTTP(w, r)
		})
	}(fs)

	go func() {
		addr := ":8085"
		cmd.Println("Starting HTTP server at http://localhost:8085/")
		err := http.ListenAndServe(addr, noCache)
		if err != nil {
			cmd.PrintErrln(fmt.Errorf("failed to start HTTP server: %w", err))
		}
	}()
}

func addListCmd(rootCmd *cobra.Command) {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List active weather warnings",
		Run: func(cmd *cobra.Command, args []string) {
			warnings, err := fetcher.FetchWarnings()
			if err != nil {
				cmd.PrintErrln(fmt.Errorf("failed to fetch warnings: %w", err))
				os.Exit(1)
			}

			if len(warnings) == 0 {
				cmd.Println("No active weather warnings.")
				return
			}

			cmd.Println("Active Weather Warnings:")
			for _, warning := range warnings {
				cmd.Println("---")
				cmd.Println(fmt.Sprintf("Type: %s", warning.Type))
				cmd.Println(fmt.Sprintf("Severity: %s", warning.Severity))
				cmd.Println(fmt.Sprintf("Area: %s", warning.Area))
				cmd.Println(fmt.Sprintf("Description: %s", warning.Description))
			}
		},
	}

	rootCmd.AddCommand(listCmd)
}
