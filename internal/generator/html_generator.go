package generator

import (
	"bytes"
	"html/template"
	"os"
	"time"

	"github.com/Zachdehooge/warnings-dashboard/internal/fetcher"
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
             --header-bg: #2d2d45;
             --header-border: #444466;
          }
          
          body {
             font-family: Arial, sans-serif;
             max-width: 1200px;
             margin: 0 auto;
             padding: 20px;
             background-color: var(--bg-color);
             color: var(--text-color);
          }
          .warnings-container {
             display: grid;
             grid-template-columns: repeat(2, 1fr);
             gap: 15px;
          }
          .warning {
             border: 1px solid var(--card-border);
             padding: 10px;
             border-radius: 5px;
             background-color: var(--card-bg);
             height: 100%;
          }
          .warning.severe {
             background-color: var(--severe-bg);
             border-color: var(--severe-border);
          }
          .warning.moderate {
             background-color: var(--moderate-bg);
             border-color: var(--moderate-border);
          }
          .warning.header {
             background-color: var(--header-bg);
             border-color: var(--header-border);
             grid-column: 1 / -1; /* Make headers span full width */
             margin-top: 15px;
             margin-bottom: 5px;
             padding: 2px 10px;
             border-width: 2px;
             height: auto;
          }
          .warning.header h2 {
             margin: 5px 0;
             text-align: center;
             font-size: 1.2em;
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
          .warning-type a {
             color: var(--text-color);
             text-decoration: none;
             transition: color 0.2s;
          }
          .warning-type a:hover {
             color: #add8e6;
             text-decoration: underline;
          }
          .back-to-top {
             position: fixed;
             bottom: 20px;
             right: 20px;
             background-color: var(--header-bg);
             color: var(--text-color);
             padding: 10px 15px;
             border-radius: 5px;
             border: 1px solid var(--header-border);
             cursor: pointer;
             text-decoration: none;
             opacity: 0.8;
             transition: opacity 0.2s;
          }
          .back-to-top:hover {
             opacity: 1;
          }
          h1, h2, h4 {
             color: var(--text-color);
          }
          .next-refresh {
             font-size: 0.8em;
             margin-top: 10px;
             color: #888;
          }
          @media (max-width: 768px) {
             .warnings-container {
                grid-template-columns: 1fr;
             }
          }
       </style>
       <script>
          // Display countdown to next refresh
          window.onload = function() {
              let refreshTime = 30; // 30 seconds
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
              
              // Show/hide back-to-top button based on scroll position
              const backToTopButton = document.querySelector('.back-to-top');
              window.addEventListener('scroll', function() {
                  if (window.scrollY > 300) {
                      backToTopButton.style.display = 'block';
                  } else {
                      backToTopButton.style.display = 'none';
                  }
              });
              
              // Initially hide the button if at the top
              if (window.scrollY <= 300) {
                  backToTopButton.style.display = 'none';
              }
          }
       </script>
    </head>
    <body>
       <h1>Active Weather Warnings</h1>
       
       {{ if .WarningTypeCounts }}
       <div class="warning-types" id="top">
          <h2>Warning Types:</h2>
          {{ range $type, $count := .WarningTypeCounts }}
             <div class="warning-type"><a href="#{{ $type | urlquery }}">{{ $type }}</a>: {{ $count }}</div>
          {{ end }}
       </div>
       {{ end }}
       
       <h4>Total Warnings: {{ .Counter }}</h4>
       <h4>Last updated: {{ .LastUpdated }}</h4>
       <div class="next-refresh">Next refresh in <span id="countdown">0:30</span></div>
       
       {{ if eq (len .Warnings) 0 }}
          <p>No active weather warnings at this time.</p>
       {{ else }}
          <div class="warnings-container">
          {{ range .Warnings }}
             {{ if eq .Severity "Header" }}
                <div class="warning header" id="{{ .Type | urlquery }}">
                   <h2>{{ .Type }}</h2>
                </div>
             {{ else }}
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
          </div>
       {{ end }}
       
       <a href="#top" class="back-to-top">↑ Top</a>
    </body>
    </html>
    `)
	if err != nil {
		return err
	}

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
		WarningTypeCounts: countWarningTypes(warnings),
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
		// Don't count our header markers as warnings
		if warning.Severity != "Header" {
			typeCounts[warning.Type]++
		}
	}
	return typeCounts
}

// TemplateWarning is a wrapper for fetcher.Warning with additional methods
type TemplateWarning struct {
	fetcher.Warning
	SeverityClass string
}

// convertWarnings transforms fetcher.Warning to TemplateWarning
// and organizes them by type for grouped display
func convertWarnings(warnings []fetcher.Warning) []TemplateWarning {
	// Group warnings by type
	warningsByType := make(map[string][]fetcher.Warning)
	var warningTypes []string

	for _, warning := range warnings {
		// If this is a new warning type, add it to our list of types
		if _, exists := warningsByType[warning.Type]; !exists {
			warningTypes = append(warningTypes, warning.Type)
		}
		warningsByType[warning.Type] = append(warningsByType[warning.Type], warning)
	}

	// Convert and add type identifier for each warning
	var templateWarnings []TemplateWarning

	for _, warningType := range warningTypes {
		typeWarnings := warningsByType[warningType]

		// Sort each type's warnings by severity
		var severeTypeWarnings, moderateTypeWarnings, otherTypeWarnings []fetcher.Warning

		for _, warning := range typeWarnings {
			switch warning.Severity {
			case "Severe", "Extreme":
				severeTypeWarnings = append(severeTypeWarnings, warning)
			case "Moderate":
				moderateTypeWarnings = append(moderateTypeWarnings, warning)
			default:
				otherTypeWarnings = append(otherTypeWarnings, warning)
			}
		}

		// Add type header marker
		headerWarning := fetcher.Warning{
			Type:        warningType,
			Severity:    "Header", // Special marker for headers
			Description: "",       // Empty description for headers
			Area:        "",
			Time:        "",
		}

		// Add header first
		templateWarnings = append(templateWarnings, TemplateWarning{
			Warning:       headerWarning,
			SeverityClass: "header",
		})

		// Then add warnings sorted by severity
		sortedTypeWarnings := append(severeTypeWarnings, append(moderateTypeWarnings, otherTypeWarnings...)...)
		for _, w := range sortedTypeWarnings {
			templateWarnings = append(templateWarnings, TemplateWarning{
				Warning:       w,
				SeverityClass: getSeverityClass(w.Severity),
			})
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
	case "Header":
		return "header"
	default:
		return ""
	}
}
