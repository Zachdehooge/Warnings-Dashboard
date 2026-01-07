package generator

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
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
             --countdown-warning: #ff4444;
             --countdown-caution: #ffaa33;
             --tornado-bg: #601a57;
             --tornado-border: #ff69b4;
             --watch-bg: #4d4d10;
             --watch-border: #aaaa00;
             --tstorm-bg: #4d0000;
             --tstorm-border: #ff0000;
             --tornado-watch-bg: #4d4d10;
             --tornado-watch-border: #ffff00;
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
             grid-template-columns: repeat(auto-fit, minmax(450px, 1fr));
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
          .warning.tornado {
             background-color: var(--tornado-bg);
             border-color: var(--tornado-border);
          }
          .warning.watch {
             background-color: var(--watch-bg);
             border-color: var(--watch-border);
          }
          .warning.tornado-watch {
             background-color: var(--tornado-watch-bg);
             border-color: var(--tornado-watch-border);
          }
          .warning.tstorm {
             background-color: var(--tstorm-bg);
             border-color: var(--tstorm-border);
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
          .warning.header.tornado-header {
             background-color: var(--tornado-bg);
             border-color: var(--tornado-border);
          }
          .warning.header.tornado-watch-header {
             background-color: var(--tornado-watch-bg);
             border-color: var(--tornado-watch-border);
          }
          .warning.header.watch-header {
             background-color: var(--watch-bg);
             border-color: var(--watch-border);
          }
          .warning.header.tstorm-header {
             background-color: var(--tstorm-bg);
             border-color: var(--tstorm-border);
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
             display: flex;
             flex-wrap: wrap;
             align-items: center;
          }
          .warning-types h2 {
             margin-right: 15px;
             margin-bottom: 5px;
          }
          .warning-type {
             margin: 5px 0;
             padding: 3px 8px;
             border-radius: 3px;
             display: inline-block;
             margin-right: 10px;
          }
          .warning-type.tornado {
             background-color: var(--tornado-bg);
             border: 1px solid var(--tornado-border);
          }
          .warning-type.tstorm {
             background-color: var(--tstorm-bg);
             border: 1px solid var(--tstorm-border);
          }
          .warning-type.tornado-watch {
             background-color: var(--tornado-watch-bg);
             border: 1px solid var(--tornado-watch-border);
          }
          .warning-type.watch {
             background-color: var(--watch-bg);
             border: 1px solid var(--watch-border);
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
          .expiration-time {
             display: flex;
             justify-content: space-between;
             align-items: center;
             margin-top: 5px;
          }
          .expiration-countdown {
             font-weight: bold;
          }
          .expiration-countdown.urgent {
             color: var(--countdown-warning);
          }
          .expiration-countdown.warning {
             color: var(--countdown-caution);
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
              
              // Initialize expiration countdowns
              updateAllExpirationCountdowns();
              
              // Update expiration countdowns every second
              setInterval(updateAllExpirationCountdowns, 1000);
          }
          
          // Function to update all expiration countdowns
          function updateAllExpirationCountdowns() {
              const expirationElements = document.querySelectorAll('[data-expires-timestamp]');
              
              expirationElements.forEach(function(element) {
                  const expiresTimestamp = parseInt(element.getAttribute('data-expires-timestamp'));
                  if (!expiresTimestamp) return;
                  
                  const now = Math.floor(Date.now() / 1000);
                  const timeLeft = expiresTimestamp - now;
                  
                  if (timeLeft <= 0) {
                      element.textContent = "EXPIRED";
                      element.classList.add("urgent");
                  } else {
                      const hours = Math.floor(timeLeft / 3600);
                      const minutes = Math.floor((timeLeft % 3600) / 60);
                      const seconds = timeLeft % 60;
                      
                      // Format the countdown
                      let countdownText = "";
                      if (hours > 0) {
                          countdownText += hours + "h ";
                      }
                      countdownText += minutes + "m " + seconds + "s";
                      
                      element.textContent = countdownText;
                      
                      // Add warning classes based on time remaining
                      element.classList.remove("urgent", "warning");
                      if (timeLeft < 1800) { // Less than 30 minutes
                          element.classList.add("urgent");
                      } else if (timeLeft < 7200) { // Less than 2 hours
                          element.classList.add("warning");
                      }
                  }
              });
          }
       </script>
    </head>
    <body>
       <h1>Active Weather Warnings</h1>
       
       {{ if .WarningTypeCounts }}
       <div class="warning-types" id="top">
          {{ range .WarningTypeCounts }}
             <div class="warning-type{{ if eq .Priority 1 }} tornado{{ else if eq .Priority 2 }} tstorm{{ else if eq .Priority 3 }} tornado-watch{{ else if eq .Priority 4 }} watch{{ end }}">
               <a href="#{{ .Type | urlquery }}">{{ .Type }}</a>: {{ .Count }}
             </div>
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
                <div class="warning header {{ .ExtraClass }}" id="{{ .Type | urlquery }}">
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
                   <small>Issued: {{ .LocalIssued }}</small><br>
                   <div class="expiration-time">
                      <small>Expires: {{ .LocalExpires }}</small>
                      <small class="expiration-countdown" data-expires-timestamp="{{ .ExpiresTimestamp }}"></small>
                   </div>
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
		WarningTypeCounts []TypeCount
	}{
		Warnings:          convertWarnings(warnings),
		LastUpdated:       time.Now().UTC().Format("Jan 2, 2006 at 03:04:01 UTC"),
		Counter:           len(warnings),
		WarningTypeCounts: sortedWarningTypeCounts(warnings),
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

// TypeCount stores count and priority info for sorting warning types
type TypeCount struct {
	Type     string
	Count    int
	Priority int
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

// sortedWarningTypeCounts creates a sorted list of warning type counts
func sortedWarningTypeCounts(warnings []fetcher.Warning) []TypeCount {
	// First count all warnings by type
	typeCounts := countWarningTypes(warnings)

	// Convert to slice for sorting
	var result []TypeCount
	for warningType, count := range typeCounts {
		result = append(result, TypeCount{
			Type:     warningType,
			Count:    count,
			Priority: getWarningTypeRank(warningType),
		})
	}

	// Sort by priority first, then by count (descending)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority // Lower priority number means higher importance
		}
		return result[i].Count > result[j].Count // Higher count is more important
	})

	return result
}

// TemplateWarning is a wrapper for fetcher.Warning with additional methods
type TemplateWarning struct {
	fetcher.Warning
	SeverityClass    string
	SeverityRank     int    // Added for sorting
	WarningTypeRank  int    // Added for prioritized ordering
	LocalIssued      string // Local time of issue
	LocalExpires     string // Local time of expiration
	ExpiresTimestamp string // Unix timestamp for JavaScript countdown
	ExtraClass       string // Extra CSS class for special warning types
}

// getWarningTypeRank assigns priority rank based on warning type
func getWarningTypeRank(warningType string) int {
	lowerType := strings.ToLower(warningType)

	// First priority: Tornado Warning
	if strings.Contains(lowerType, "tornado warning") {
		return 1
	}
	// Second priority: Severe Thunderstorm Warning
	if strings.Contains(lowerType, "thunderstorm warning") ||
		strings.Contains(lowerType, "t-storm warning") ||
		strings.Contains(lowerType, "tstorm warning") {
		return 2
	}
	// Third priority: Tornado Watch
	if strings.Contains(lowerType, "tornado") && strings.Contains(lowerType, "watch") {
		return 3
	}
	// Fourth priority: Severe Thunderstorm Watch
	if (strings.Contains(lowerType, "thunderstorm") ||
		strings.Contains(lowerType, "t-storm") ||
		strings.Contains(lowerType, "tstorm")) &&
		strings.Contains(lowerType, "watch") {
		return 4
	}
	// All other warnings
	return 5
}

// convertWarnings transforms fetcher.Warning to TemplateWarning
// and sorts all warnings by type priority first, then by severity
func convertWarnings(warnings []fetcher.Warning) []TemplateWarning {
	// Group warnings by type
	warningsByType := make(map[string][]TemplateWarning)
	var warningTypes []string

	// First convert all warnings to TemplateWarning
	for _, warning := range warnings {
		// Format the times to local time - ensure both times are properly converted to local time
		localIssued := formatToLocalTime(warning.Time)
		localExpires := formatToLocalTime(warning.ExpiresTime)

		// Get Unix timestamp for expiration countdown
		expiresTimestamp := getExpiresTimestamp(warning.ExpiresTime)

		// Determine if this is a special warning type
		lowerType := strings.ToLower(warning.Type)
		isTornado := strings.Contains(lowerType, "tornado warning")
		isTornadoWatch := strings.Contains(lowerType, "tornado") && strings.Contains(lowerType, "watch")
		isThunderstormWatch := (strings.Contains(lowerType, "thunderstorm") ||
			strings.Contains(lowerType, "t-storm") ||
			strings.Contains(lowerType, "tstorm")) &&
			strings.Contains(lowerType, "watch")
		isTstorm := strings.Contains(lowerType, "thunderstorm warning") ||
			strings.Contains(lowerType, "t-storm warning") ||
			strings.Contains(lowerType, "tstorm warning")

		// Determine CSS class based on warning type and severity
		var severityClass string
		if isTornadoWatch {
			severityClass = "tornado-watch"
		} else if isThunderstormWatch {
			severityClass = "watch"
		} else if isTornado {
			severityClass = "tornado"
		} else if isTstorm {
			severityClass = "tstorm"
		} else {
			severityClass = getSeverityClass(warning.Severity)
		}

		// Get type rank for prioritized ordering
		warningTypeRank := getWarningTypeRank(warning.Type)

		templateWarning := TemplateWarning{
			Warning:          warning,
			SeverityClass:    severityClass,
			SeverityRank:     getSeverityRank(warning.Severity),
			WarningTypeRank:  warningTypeRank,
			LocalIssued:      localIssued,
			LocalExpires:     localExpires,
			ExpiresTimestamp: expiresTimestamp,
			ExtraClass:       "", // Will be set for headers later
		}

		// If this is a new warning type, add it to our list of types
		if _, exists := warningsByType[warning.Type]; !exists {
			warningTypes = append(warningTypes, warning.Type)
		}
		warningsByType[warning.Type] = append(warningsByType[warning.Type], templateWarning)
	}

	// Sort each type's warnings by severity
	for warningType := range warningsByType {
		sort.Slice(warningsByType[warningType], func(i, j int) bool {
			return warningsByType[warningType][i].SeverityRank > warningsByType[warningType][j].SeverityRank
		})
	}

	// Sort warning types by our predetermined priority
	sort.Slice(warningTypes, func(i, j int) bool {
		// Get the warning type rank for each type
		iTypeRank := getWarningTypeRank(warningTypes[i])
		jTypeRank := getWarningTypeRank(warningTypes[j])

		// First sort by warning type rank
		if iTypeRank != jTypeRank {
			return iTypeRank < jTypeRank // Lower rank number = higher priority
		}

		// If same warning type rank, sort by highest severity
		iMaxSeverity := 0
		for _, w := range warningsByType[warningTypes[i]] {
			if w.SeverityRank > iMaxSeverity {
				iMaxSeverity = w.SeverityRank
			}
		}

		jMaxSeverity := 0
		for _, w := range warningsByType[warningTypes[j]] {
			if w.SeverityRank > jMaxSeverity {
				jMaxSeverity = w.SeverityRank
			}
		}

		return iMaxSeverity > jMaxSeverity
	})

	// Build the final ordered list of warnings
	var templateWarnings []TemplateWarning

	for _, warningType := range warningTypes {
		typeWarnings := warningsByType[warningType]

		// Check if this is a special warning type
		lowerType := strings.ToLower(warningType)
		isTornadoWarning := strings.Contains(lowerType, "tornado warning")
		isTornadoWatch := strings.Contains(lowerType, "tornado") && strings.Contains(lowerType, "watch")
		isThunderstormWatch := (strings.Contains(lowerType, "thunderstorm") ||
			strings.Contains(lowerType, "t-storm") ||
			strings.Contains(lowerType, "tstorm")) &&
			strings.Contains(lowerType, "watch")
		isTstormWarning := strings.Contains(lowerType, "thunderstorm warning") ||
			strings.Contains(lowerType, "t-storm warning") ||
			strings.Contains(lowerType, "tstorm warning")

		// Set extra class for special headers
		extraHeaderClass := ""
		if isTornadoWarning {
			extraHeaderClass = "tornado-header"
		} else if isTornadoWatch {
			extraHeaderClass = "tornado-watch-header"
		} else if isThunderstormWatch {
			extraHeaderClass = "watch-header"
		} else if isTstormWarning {
			extraHeaderClass = "tstorm-header"
		}

		// Add type header marker
		headerWarning := TemplateWarning{
			Warning: fetcher.Warning{
				Type:        warningType,
				Severity:    "Header", // Special marker for headers
				Description: "",       // Empty description for headers
				Area:        "",
				Time:        "",
			},
			SeverityClass:    "header",
			SeverityRank:     0,
			WarningTypeRank:  getWarningTypeRank(warningType),
			LocalIssued:      "",
			LocalExpires:     "",
			ExpiresTimestamp: "",
			ExtraClass:       extraHeaderClass,
		}

		// Add header first
		templateWarnings = append(templateWarnings, headerWarning)

		// Then add warnings (already sorted by severity)
		templateWarnings = append(templateWarnings, typeWarnings...)
	}

	return templateWarnings
}

// formatToLocalTime converts time strings to local time
func formatToLocalTime(timeStr string) string {
	// If the time string is empty, return an appropriate message
	if timeStr == "" {
		return "Not specified"
	}

	// Parse the input time string based on the expected format
	t, err := time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err != nil {
		// Try alternative date format that might be used
		t, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			// If there's still an error parsing, just return the original string
			return timeStr
		}
	}

	// Convert to local time zone
	localTime := t.Local()

	// Format the time in a user-friendly way
	return localTime.Format("Jan 2, 2006 at 3:04 PM MST")
}

// getExpiresTimestamp converts expiration time to Unix timestamp for JavaScript countdown
func getExpiresTimestamp(timeStr string) string {
	if timeStr == "" {
		return ""
	}

	// Parse the input time string based on the expected format
	t, err := time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err != nil {
		// Try alternative date format that might be used
		t, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			// If there's an error parsing, return empty string
			return ""
		}
	}

	// Convert to Unix timestamp (seconds since epoch)
	return fmt.Sprintf("%d", t.Unix())
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

// getSeverityRank returns a numeric rank for sorting warnings by severity
func getSeverityRank(severity string) int {
	switch severity {
	case "Extreme":
		return 4
	case "Severe":
		return 3
	case "Moderate":
		return 2
	case "Minor":
		return 1
	case "Header": // Special case for our header markers
		return 0
	default:
		return 0
	}
}
