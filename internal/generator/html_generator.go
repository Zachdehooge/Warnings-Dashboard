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
       <meta http-equiv="refresh" content="30"> <!-- Auto-refresh every 30 seconds -->
       <title>US Weather Warnings</title>
       <style>
          :root {
             --bg-color: #121212;
             --text-color: #e0e0e0;
             --card-bg: #1e1e1e;
             --card-border: #333;
             --severe-bg: #3d1a1a;
             --severe-border: #a52a2a;
             --moderate-bg: #3d2e1a;
             --moderate-border: #b25900;
             --summary-bg: #252525;
          }
          
          body {
             font-family: Arial, sans-serif;
             max-width: 800px;
             margin: 0 auto;
             padding: 20px;
             background-color: var(--bg-color);
             color: var(--text-color);
          }
          .warning {
             border: 1px solid var(--card-border);
             margin-bottom: 15px;
             padding: 10px;
             border-radius: 5px;
             background-color: var(--card-bg);
          }
          .warning.severe {
             background-color: var(--severe-bg);
             border-color: var(--severe-border);
          }
          .warning.moderate {
             background-color: var(--moderate-bg);
             border-color: var(--moderate-border);
          }
          .warning-header {
             display: flex;
             justify-content: space-between;
             align-items: center;
          }
          .warning-types {
             margin-bottom: 15px;
             background-color: var(--summary-bg);
             padding: 15px;
             border-radius: 5px;
          }
          .warning-type {
             margin: 5px 0;
          }
          h1, h2, h4 {
             color: var(--text-color);
          }
          .next-refresh {
             font-size: 0.8em;
             margin-top: 10px;
             color: #888;
          }
       </style>
       <script>
          // Display countdown to next refresh
          window.onload = function() {
              let refreshTime = 30; // 5 minutes in seconds
              const countdownElement = document.getElementById('countdown');
              
              setInterval(function() {
                  refreshTime--;
                  const minutes = Math.floor(refreshTime / 60);
                  const seconds = refreshTime % 60;
                  countdownElement.textContent = minutes + ':' + (seconds < 10 ? '0' : '') + seconds;
                  
                  if (refreshTime <= 0) {
                      countdownElement.textContent = "Refreshing...";
                  }
              }, 1000);
          }
       </script>
    </head>
    <body>
       <h1>Active Weather Warnings</h1>
       
       {{ if .WarningTypeCounts }}
       <div class="warning-types">
          <h2>Warning Types:</h2>
          {{ range $type, $count := .WarningTypeCounts }}
             <div class="warning-type">{{ $type }}: {{ $count }}</div>
          {{ end }}
       </div>
       {{ end }}
       
       <h4>Total Warnings: {{ .Counter }}</h4>
       <h4>Last updated: {{ .LastUpdated }}</h4>
       <div class="next-refresh">Next refresh in <span id="countdown">0:30</span></div>
       
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

	// Count warning types
	warningTypeCounts := countWarningTypes(warnings)

	// Prepare data for template
	data := struct {
		Warnings          []TemplateWarning
		LastUpdated       string
		Counter           int
		WarningTypeCounts map[string]int
	}{
		Warnings:          convertWarnings(warnings),
		LastUpdated:       time.Now().Format("Jan 2, 2006 at 3:04:01 PM"),
		Counter:           len(warnings),
		WarningTypeCounts: warningTypeCounts,
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

// countWarningTypes counts the number of each type of warning
func countWarningTypes(warnings []fetcher.Warning) map[string]int {
	typeCounts := make(map[string]int)
	for _, warning := range warnings {
		typeCounts[warning.Type]++
	}
	return typeCounts
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
