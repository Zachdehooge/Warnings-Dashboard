package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Warning represents a weather warning
type Warning struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Area        string `json:"area"`
	Severity    string `json:"severity"`
	Time        string `json:"time"`
	ExpiresTime string `json:"expires_time"` // Added expires time field
}

// FetchWarnings retrieves weather warnings from the National Weather Service API
func FetchWarnings() ([]Warning, error) {
	// NWS API endpoint for active alerts
	url := "https://api.weather.gov/alerts/active"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch warnings: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON response
	var apiResponse struct {
		Features []struct {
			Properties struct {
				ID          string `json:"id"`
				Event       string `json:"event"`
				Description string `json:"description"`
				Severity    string `json:"severity"`
				Sent        string `json:"sent"`
				Expires     string `json:"expires"` // Added expires field
				Headline    string `json:"headline"`
				Area        string `json:"areaDesc"`
			} `json:"properties"`
		} `json:"features"`
	}

	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Transform API response to our Warning struct
	warnings := make([]Warning, 0, len(apiResponse.Features))
	for _, feature := range apiResponse.Features {
		// Filter out unwanted warning types
		if isFilteredWarning(feature.Properties.Event) {
			continue
		}

		if isFilteredWords(feature.Properties.Description) {
			continue
		}

		warnings = append(warnings, Warning{
			ID:          feature.Properties.ID,
			Type:        feature.Properties.Event,
			Description: feature.Properties.Description,
			Area:        feature.Properties.Area,
			Severity:    feature.Properties.Severity,
			Time:        feature.Properties.Sent,
			ExpiresTime: feature.Properties.Expires, // Store the expires time
		})
	}

	return warnings, nil
}

func isFilteredWords(words string) bool {
	words = strings.ToLower(words)

	filteredTypes := []string{
		"fire",
	}

	for _, filteredType := range filteredTypes {
		if strings.Contains(words, filteredType) {
			return true
		}
	}

	return false
}

// isFilteredWarning checks if a warning should be filtered out
func isFilteredWarning(eventType string) bool {
	// Convert to lowercase for case-insensitive matching
	lowercaseEvent := strings.ToLower(eventType)

	// ! UNCOMMENT BELOW TO KEEP WINTER STORM WARNINGS
	/*if strings.Contains(lowercaseEvent, "winter storm") {
	   return false
	}*/

	if strings.Contains(lowercaseEvent, "severe thunderstorm") {
		return false
	}

	if strings.Contains(lowercaseEvent, "tornado") {
		return false
	}

	// ! List of warning types to filter out
	filteredTypes := []string{
		"storm warning",
		"special weather statement",
		"storm watch",
		"flood",
		"winter weather advisory",
		"extreme heat warning",
		"frost advisory",
		"freeze warning",
		"gale warning",
		"test message",
		"high wind warning",
		"wind advisory",
		"high wind watch",
		"heat advisory",
		"dense fog advisory",
		"blowing dust advisory",
		"dust storm warning",
		"small craft advisory",
		"red flag warning",
		"air quality alert",
		"heavy freezing spray warning",
		"fire weather watch",
		"gale watch",
		"blowing dust warning",
		"hydrologic outlook",
		"marine",
		"coastal flood",
		"river flood",
		"flash flood",
		"high surf",
		"rip current",
		"beach hazard",
		"coastal hazard",
		"coastal erosion",
	}

	// Check if the event type contains any filtered keywords
	for _, filteredType := range filteredTypes {
		if strings.Contains(lowercaseEvent, filteredType) {
			return true
		}
	}

	return false
}
