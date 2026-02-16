package generator

import (
	"bytes"
	"encoding/json"
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
	tmpl, err := template.New("warnings").Funcs(template.FuncMap{
		"toJSON": toJSON,
	}).Parse(`
    <!DOCTYPE html>
    <html lang="en">
    <head>
       <meta charset="UTF-8"/>
       <meta http-equiv="refresh" content="30"> <!-- Auto-refresh every 30 seconds -->
       <title>US Weather Warnings</title>
       <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />
       <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
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
             --tab-active-bg: #3d3d5c;
          }
          
          body {
             font-family: Arial, sans-serif;
             max-width: 1200px;
             margin: 0 auto;
             padding: 20px;
             background-color: var(--bg-color);
             color: var(--text-color);
          }
          
          .tabs {
             display: flex;
             gap: 10px;
             margin-bottom: 20px;
             border-bottom: 2px solid var(--card-border);
          }
          
          .tab {
             padding: 10px 20px;
             background-color: var(--card-bg);
             border: 1px solid var(--card-border);
             border-bottom: none;
             border-radius: 5px 5px 0 0;
             cursor: pointer;
             transition: background-color 0.2s;
          }
          
          .tab:hover {
             background-color: var(--header-bg);
          }
          
          .tab.active {
             background-color: var(--tab-active-bg);
             border-color: var(--header-border);
          }
          
          .tab-content {
             display: none;
          }
          
          .tab-content.active {
             display: block;
          }
          
          #map {
             height: 600px;
             width: 100%;
             border: 2px solid var(--card-border);
             border-radius: 5px;
             margin-top: 20px;
          }
          
          .map-legend {
             background-color: var(--card-bg);
             padding: 10px;
             border-radius: 5px;
             margin-top: 10px;
             border: 1px solid var(--card-border);
          }
          
          .legend-item {
             display: flex;
             align-items: center;
             margin: 5px 0;
          }
          
          .legend-color {
             width: 30px;
             height: 20px;
             margin-right: 10px;
             border: 1px solid #fff;
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
          let map;
          let warningsData = {{ .WarningsJSON }};
          let countyBoundaries = null; // Will store county boundary data
          
          // Load county boundaries from NWS GeoJSON
          async function loadCountyBoundaries() {
              try {
                  // Using simplified US counties GeoJSON from a CDN
                  const response = await fetch('https://raw.githubusercontent.com/plotly/datasets/master/geojson-counties-fips.json');
                  countyBoundaries = await response.json();
                  console.log('County boundaries loaded successfully');
              } catch (error) {
                  console.error('Failed to load county boundaries:', error);
                  console.log('County fallback will not be available');
              }
          }
          
          // Display countdown to next refresh
          window.onload = function() {
              // Restore the previously active tab FIRST, before any other initialization
              restoreActiveTab();
              
              // Set up refresh countdown
              let refreshTime = 30; // 30 seconds
              const countdownElements = document.querySelectorAll('.countdown');
              
              setInterval(function() {
                  refreshTime--;
                  const minutes = Math.floor(refreshTime / 60);
                  const seconds = refreshTime % 60;
                  const timeString = minutes + ':' + (seconds < 10 ? '0' : '') + seconds;
                  
                  countdownElements.forEach(function(element) {
                      element.textContent = timeString;
                  });
                  
                  if (refreshTime <= 0) {
                      countdownElements.forEach(function(element) {
                          element.textContent = "Refreshing...";
                      });
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
          
          // Function to switch tabs
          function switchTab(tabName) {
              // Hide all tab contents
              const tabContents = document.querySelectorAll('.tab-content');
              tabContents.forEach(content => content.classList.remove('active'));
              
              // Remove active class from all tabs
              const tabs = document.querySelectorAll('.tab');
              tabs.forEach(tab => tab.classList.remove('active'));
              
              // Show selected tab content
              document.getElementById(tabName + '-content').classList.add('active');
              
              // Add active class to selected tab
              event.target.classList.add('active');
              
              // Save the active tab to localStorage
              localStorage.setItem('activeTab', tabName);
              
              // Initialize map if switching to map tab
              if (tabName === 'map' && !map) {
                  initMap();
              }
          }
          
          // Restore the previously active tab
          function restoreActiveTab() {
              const savedTab = localStorage.getItem('activeTab');
              
              // If there's a saved tab preference, switch to it
              if (savedTab && (savedTab === 'list' || savedTab === 'map')) {
                  // Hide all tab contents
                  const tabContents = document.querySelectorAll('.tab-content');
                  tabContents.forEach(content => content.classList.remove('active'));
                  
                  // Remove active class from all tabs
                  const tabs = document.querySelectorAll('.tab');
                  tabs.forEach(tab => tab.classList.remove('active'));
                  
                  // Show the saved tab content
                  document.getElementById(savedTab + '-content').classList.add('active');
                  
                  // Add active class to the corresponding tab button
                  const tabButtons = document.querySelectorAll('.tab');
                  tabButtons.forEach(tab => {
                      if ((savedTab === 'list' && tab.textContent === 'List View') ||
                          (savedTab === 'map' && tab.textContent === 'Map View')) {
                          tab.classList.add('active');
                      }
                  });
                  
                  // Initialize map if the saved tab is map
                  if (savedTab === 'map') {
                      initMap();
                  }
              }
          }
          
          // Initialize the map
          async function initMap() {
              // Create map centered on continental US
              map = L.map('map').setView([39.8283, -98.5795], 4);
              
              // Add OpenStreetMap tile layer with dark theme
              L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
                  attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
                  subdomains: 'abcd',
                  maxZoom: 20
              }).addTo(map);
              
              // Load county boundaries if not already loaded
              if (!countyBoundaries) {
                  await loadCountyBoundaries();
              }
              
              // Add warnings to map
              addWarningsToMap();
          }
          
          // Get color based on warning type
          function getWarningColor(warningType, severity) {
              const lowerType = warningType.toLowerCase();
              
              if (lowerType.includes('tornado warning')) {
                  return '#ff69b4'; // Pink for tornado warnings
              } else if (lowerType.includes('tornado') && lowerType.includes('watch')) {
                  return '#ffff00'; // Yellow for tornado watch
              } else if (lowerType.includes('thunderstorm warning') || 
                         lowerType.includes('t-storm warning') || 
                         lowerType.includes('tstorm warning')) {
                  return '#ff0000'; // Red for severe thunderstorm warning
              } else if ((lowerType.includes('thunderstorm') || 
                         lowerType.includes('t-storm') || 
                         lowerType.includes('tstorm')) && 
                         lowerType.includes('watch')) {
                  return '#aaaa00'; // Dark yellow for thunderstorm watch
              } else if (severity === 'Severe' || severity === 'Extreme') {
                  return '#a52a2a'; // Brown for other severe warnings
              } else if (severity === 'Moderate') {
                  return '#b25900'; // Orange for moderate warnings
              } else {
                  return '#666666'; // Gray for other warnings
              }
          }
          
          // Add warnings to the map
          function addWarningsToMap() {
              let addedCount = 0;
              let skippedCount = 0;
              let countyFallbackCount = 0;
              
              warningsData.forEach(warning => {
                  // Skip header entries
                  if (warning.Severity === 'Header') return;
                  
                  const color = getWarningColor(warning.Type, warning.Severity);
                  
                  try {
                      // First try to use the actual polygon geometry
                      if (warning.Geometry && warning.Geometry.type) {
                          const geometry = warning.Geometry;
                          
                          // Handle different geometry types
                          if (geometry.type === 'Polygon') {
                              addPolygonToMap(geometry.coordinates, warning, color);
                              addedCount++;
                          } else if (geometry.type === 'MultiPolygon') {
                              geometry.coordinates.forEach(polygonCoords => {
                                  addPolygonToMap(polygonCoords, warning, color);
                              });
                              addedCount++;
                          } else {
                              console.log('Unknown geometry type:', geometry.type, 'for warning:', warning.Type);
                              skippedCount++;
                          }
                      } 
                      // Fallback to county boundaries if no geometry but we have SAME codes
                      else if (warning.SAME && warning.SAME.length > 0 && countyBoundaries) {
                          const added = addCountyFallback(warning, color);
                          if (added) {
                              countyFallbackCount++;
                              addedCount++;
                          } else {
                              console.log('Warning missing geometry and county fallback failed:', warning.Type, warning.Area);
                              skippedCount++;
                          }
                      }
                      else {
                          console.log('Warning missing geometry:', warning.Type, warning.Area);
                          skippedCount++;
                      }
                  } catch (error) {
                      console.error('Error adding warning to map:', warning.Type, error);
                      skippedCount++;
                  }
              });
              
              console.log('Map loaded:', addedCount, 'warnings added,', skippedCount, 'skipped');
              if (countyFallbackCount > 0) {
                  console.log('County fallback used for', countyFallbackCount, 'warnings');
              }
          }
          
          // Add a polygon to the map
          function addPolygonToMap(coordinates, warning, color) {
              try {
                  // Convert coordinates from [lng, lat] to [lat, lng] for Leaflet
                  const latLngs = coordinates[0].map(coord => [coord[1], coord[0]]);
                  
                  // Create polygon
                  const polygon = L.polygon(latLngs, {
                      color: color,
                      fillColor: color,
                      fillOpacity: 0.3,
                      weight: 2
                  }).addTo(map);
                  
                  // Add popup with warning details
                  const popupContent = '<div style="color: #000;">' +
                      '<h3>' + warning.Type + '</h3>' +
                      '<p><strong>Severity:</strong> ' + warning.Severity + '</p>' +
                      '<p><strong>Area:</strong> ' + warning.Area + '</p>' +
                      '<p><strong>Expires:</strong> ' + formatTime(warning.ExpiresTime) + '</p>' +
                      '</div>';
                  
                  polygon.bindPopup(popupContent);
                  
                  console.log('Added polygon for:', warning.Type, 'in', warning.Area);
              } catch (error) {
                  console.error('Error creating polygon for', warning.Type, ':', error);
                  console.log('Coordinates:', coordinates);
              }
          }
          
          // Add county boundaries as fallback when specific polygon is not available
          function addCountyFallback(warning, color) {
              if (!countyBoundaries || !warning.SAME || warning.SAME.length === 0) {
                  return false;
              }
              
              let addedAny = false;
              
              // Convert SAME codes to FIPS format (SAME format is SSCCC where SS is state, CCC is county)
              warning.SAME.forEach(sameCode => {
                  // SAME code format: first 3 digits are state FIPS (with leading 0), last 3 are county FIPS
                  // We need it as 5 digits total for matching
                  const fipsCode = sameCode.substring(1); // Remove leading 0 to get standard 5-digit FIPS
                  
                  // Find matching county in the GeoJSON
                  const county = countyBoundaries.features.find(feature => 
                      feature.id === fipsCode || feature.properties.GEO_ID === fipsCode
                  );
                  
                  if (county && county.geometry) {
                      try {
                          // Add the county boundary to the map
                          const geoJsonLayer = L.geoJSON(county, {
                              style: {
                                  color: color,
                                  fillColor: color,
                                  fillOpacity: 0.2, // Lighter opacity for county fallback
                                  weight: 2,
                                  dashArray: '5, 5' // Dashed line to indicate it's a county boundary, not exact polygon
                              }
                          }).addTo(map);
                          
                          // Add popup
                          const popupContent = '<div style="color: #000;">' +
                              '<h3>' + warning.Type + '</h3>' +
                              '<p><strong>Severity:</strong> ' + warning.Severity + '</p>' +
                              '<p><strong>Area:</strong> ' + warning.Area + '</p>' +
                              '<p><strong>Expires:</strong> ' + formatTime(warning.ExpiresTime) + '</p>' +
                              '<p style="font-style: italic; font-size: 0.9em;">Note: Showing county boundary (exact polygon unavailable)</p>' +
                              '</div>';
                          
                          geoJsonLayer.bindPopup(popupContent);
                          addedAny = true;
                          
                          console.log('Added county fallback for:', warning.Type, 'FIPS:', fipsCode);
                      } catch (error) {
                          console.error('Error adding county boundary:', error);
                      }
                  }
              });
              
              return addedAny;
          }
          
          // Format time for display
          function formatTime(timeStr) {
              if (!timeStr) return 'Not specified';
              const date = new Date(timeStr);
              return date.toLocaleString();
          }
          
          // Show debug information
          function showDebugInfo() {
              const debugDiv = document.getElementById('debug-info');
              if (debugDiv.style.display === 'none') {
                  let info = 'Total warnings in data: ' + warningsData.length + '\n\n';
                  
                  warningsData.forEach((warning, index) => {
                      info += (index + 1) + '. ' + warning.Type + ' - ' + warning.Area + '\n';
                      info += '   Severity: ' + warning.Severity + '\n';
                      info += '   Has Geometry: ' + (warning.Geometry ? 'Yes' : 'NO') + '\n';
                      if (warning.Geometry) {
                          info += '   Geometry Type: ' + (warning.Geometry.type || 'unknown') + '\n';
                      }
                      if (warning.SAME && warning.SAME.length > 0) {
                          info += '   SAME Codes: ' + warning.SAME.join(', ') + '\n';
                          info += '   County Fallback: ' + (countyBoundaries ? 'Available' : 'Unavailable') + '\n';
                      }
                      info += '\n';
                  });
                  
                  debugDiv.textContent = info;
                  debugDiv.style.display = 'block';
              } else {
                  debugDiv.style.display = 'none';
              }
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
       
       <!-- Tab Navigation -->
       <div class="tabs">
          <div class="tab active" onclick="switchTab('list')">List View</div>
          <div class="tab" onclick="switchTab('map')">Map View</div>
       </div>
       
       <!-- List Tab Content -->
       <div id="list-content" class="tab-content active">
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
          <div class="next-refresh">Next refresh in <span class="countdown">0:30</span></div>
          
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
       </div>
       
       <!-- Map Tab Content -->
       <div id="map-content" class="tab-content">
          <h4>Total Warnings: {{ .Counter }}</h4>
          <h4>Last updated: {{ .LastUpdated }}</h4>
          <div class="next-refresh">Next refresh in <span class="countdown">0:30</span></div>
          <button onclick="showDebugInfo()" style="margin-bottom: 10px; padding: 5px 10px; background-color: var(--header-bg); color: var(--text-color); border: 1px solid var(--header-border); border-radius: 3px; cursor: pointer;">Show Debug Info</button>
          <div id="debug-info" style="display: none; background-color: var(--summary-bg); padding: 10px; margin-bottom: 10px; border-radius: 5px; font-family: monospace; font-size: 0.9em; max-height: 200px; overflow-y: auto;"></div>
          <div id="map"></div>
          <div class="map-legend">
             <h3>Legend</h3>
             <div class="legend-item">
                <div class="legend-color" style="background-color: #ff69b4;"></div>
                <span>Tornado Warning</span>
             </div>
             <div class="legend-item">
                <div class="legend-color" style="background-color: #ff0000;"></div>
                <span>Severe Thunderstorm Warning</span>
             </div>
             <div class="legend-item">
                <div class="legend-color" style="background-color: #ffff00;"></div>
                <span>Tornado Watch</span>
             </div>
             <div class="legend-item">
                <div class="legend-color" style="background-color: #aaaa00;"></div>
                <span>Severe Thunderstorm Watch</span>
             </div>
             <div class="legend-item">
                <div class="legend-color" style="background-color: #a52a2a;"></div>
                <span>Other Severe Warnings</span>
             </div>
             <div class="legend-item">
                <div class="legend-color" style="background-color: #b25900;"></div>
                <span>Moderate Warnings</span>
             </div>
             <div style="margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--card-border); font-size: 0.9em; color: #888;">
                <strong>Note:</strong> Solid polygons show exact warning boundaries. Dashed lines indicate county boundaries (used when exact polygon unavailable).
             </div>
          </div>
       </div>
       
       <a href="#top" class="back-to-top">↑ Top</a>
    </body>
    </html>
    `)
	if err != nil {
		return err
	}

	// Convert warnings to JSON for JavaScript
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return fmt.Errorf("failed to marshal warnings to JSON: %w", err)
	}

	// Prepare data for template
	data := struct {
		Warnings          []TemplateWarning
		LastUpdated       string
		Counter           int
		WarningTypeCounts []TypeCount
		WarningsJSON      template.JS
	}{
		Warnings:          convertWarnings(warnings),
		LastUpdated:       time.Now().UTC().Format("Jan 2, 2006 at 03:04:01 UTC"),
		Counter:           len(warnings),
		WarningTypeCounts: sortedWarningTypeCounts(warnings),
		WarningsJSON:      template.JS(warningsJSON),
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

// toJSON converts data to JSON for use in templates
func toJSON(v interface{}) (template.JS, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return template.JS(b), nil
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
