package generator

import (
	"bytes"
	"html/template"
	"os"
	"time"

	"github.com/Zachdehooge/web-weather-dashboard/internal/fetcher"
)

// GenerateWarningsHTML creates an HTML file with weather warnings
func GenerateWarningsHTML(warnings []fetcher.Warning, outputPath string) error {
	// Define the HTML template
	tmpl, err := template.New("warnings").Parse(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8"/>
		<meta http-equiv="refresh" content="10">
		<title>US Weather Warnings</title>
		<style>
			body {
				font-family: Arial, sans-serif;
				max-width: 800px;
				margin: 0 auto;
				padding: 20px;
			}
			.warning {
				border: 1px solid #ddd;
				margin-bottom: 15px;
				padding: 10px;
				border-radius: 5px;
			}
			.warning.severe {
				background-color: #ffdddd;
				border-color: #ff0000;
			}
			.warning.moderate {
				background-color: #fff4dd;
				border-color: #ffa500;
			}
			.warning-header {
				display: flex;
				justify-content: space-between;
				align-items: center;
			}
		</style>
	</head>
	<body>
		<h1>Active Weather Warnings</h1>
		<p>Current Warnings: {{ .Counter }}</p>
		<p>Last updated: {{ .LastUpdated }}</p>
		
		{{ if eq (len .Warnings) 0 }}
			<p>No active weather warnings at this time.</p>
		{{ else }}
			{{ range .Warnings }}
				<div class="warning {{ .SeverityClass }}">
					<div class="warning-header">
						<h2>{{ .Type }}</h2>
						<strong>{{ .Severity }} Severity</strong>
					</div>
					<p><strong>Area:</strong> {{ .Area }}</p>
					<p>{{ .Description }}</p>
					<small>Issued: {{ .Time }}</small>
				</div>
			{{ end }}
		{{ end }}
	</body>
	</html>
	`)
	if err != nil {
		return err
	}

	// Prepare data for template
	data := struct {
		Warnings    []TemplateWarning
		LastUpdated string
		Counter     int
	}{
		Warnings:    convertWarnings(warnings),
		LastUpdated: time.Now().Format("Jan 2, 2006 at 3:04 PM"),
		Counter:     len(warnings),
	}

	// Create a buffer to store the rendered HTML
	var buf bytes.Buffer

	// Execute the template
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// TemplateWarning is a wrapper for fetcher.Warning with additional methods
type TemplateWarning struct {
	fetcher.Warning
	SeverityClass string
}

// convertWarnings transforms fetcher.Warning to TemplateWarning
func convertWarnings(warnings []fetcher.Warning) []TemplateWarning {
	templateWarnings := make([]TemplateWarning, len(warnings))
	for i, w := range warnings {
		templateWarnings[i] = TemplateWarning{
			Warning:       w,
			SeverityClass: getSeverityClass(w.Severity),
		}
	}
	return templateWarnings
}

// getSeverityClass determines the CSS class based on severity
func getSeverityClass(severity string) string {
	switch severity {
	case "Severe", "Extreme":
		return "severe"
	case "Moderate":
		return "moderate"
	default:
		return ""
	}
}
