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
	// Also generate JSON file for AJAX updates
	jsonPath := strings.Replace(outputPath, ".html", ".json", 1)
	if err := GenerateWarningsJSON(warnings, jsonPath); err != nil {
		return fmt.Errorf("failed to generate JSON: %w", err)
	}

	// Define the HTML template
	tmpl, err := template.New("warnings").Funcs(template.FuncMap{
		"toJSON": toJSON,
	}).Parse(`
    <!DOCTYPE html>
    <html lang="en">
    <head>
       <meta charset="UTF-8"/>
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
             --mcd-bg: #1a3d3d;
             --mcd-border: #00aaaa;
          }
          
          body {
             font-family: Arial, sans-serif;
             max-width: 1200px;
             margin: 0 auto;
             padding: 20px;
             background-color: var(--bg-color);
             color: var(--text-color);
             /* Smooth fade-in on page load */
             animation: fadeIn 0.3s ease-in;
          }
          
          @keyframes fadeIn {
             from { opacity: 0; }
             to { opacity: 1; }
          }
          
          /* Prevent flash of white during reload */
          html {
             background-color: #121212;
          }
          
          .warning-title:hover {
             text-decoration: underline;
             color: #add8e6;
          }
          
          #map {
             height: 600px;
             width: 100%;
             border: 2px solid var(--card-border);
             border-radius: 5px;
             margin-top: 20px;
             /* Smooth appearance */
             opacity: 1;
             transition: opacity 0.2s ease-in-out;
          }
          
          #map.loading {
             opacity: 0.7;
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
          .warning.mcd {
             background-color: var(--mcd-bg);
             border-color: var(--mcd-border);
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
          .warning-type.mcd {
             background-color: var(--mcd-bg);
             border: 1px solid var(--mcd-border);
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
          let warningLayers = []; // Track all warning layers for cleanup
          let mesoscaleDiscussions = []; // Store mesoscale discussions
          let updateInterval = 30000; // 30 seconds in milliseconds
          let lastUpdateTime = Date.now();
          
          // Fetch mesoscale discussions from SPC
          async function fetchMesoscaleDiscussions() {
              try {
                  console.log('Fetching mesoscale discussions...');
                  
                  // Try the Iowa State Archive GeoJSON endpoint - it includes geometry
                  let url = 'https://mesonet.agron.iastate.edu/geojson/spc_mcd.geojson';
                  let response = await fetch(url);
                  
                  console.log('IEM MCD endpoint status:', response.status);
                  
                  // If that fails, try the direct SPC endpoint
                  if (!response.ok) {
                      console.log('IEM endpoint failed, trying SPC direct...');
                      url = 'https://www.spc.noaa.gov/products/md/mcd.geojson';
                      response = await fetch(url);
                      console.log('SPC MCD endpoint status:', response.status);
                  }
                  
                  // If that also fails, try the MapServer endpoint
                  if (!response.ok) {
                      console.log('SPC endpoint failed, trying MapServer...');
                      url = 'https://mapservices.weather.noaa.gov/vector/rest/services/outlooks/spc_mesoscale_discussion/MapServer/0/query?where=1%3D1&outFields=*&f=geojson';
                      response = await fetch(url);
                      console.log('MapServer MCD endpoint status:', response.status);
                  }
                  
                  if (!response.ok) {
                      throw new Error('Failed to fetch mesoscale discussions: ' + response.status);
                  }
                  
                  const data = await response.json();
                  console.log('MCD Response data:', data);
                  console.log('MCD Features:', data.features);
                  console.log('Fetched', (data.features || []).length, 'mesoscale discussions');
                  
                  // Filter out features with null geometry
                  const validFeatures = (data.features || []).filter(feature => {
                      if (!feature.geometry || feature.geometry === null) {
                          console.log('Filtered out MCD with null geometry:', feature);
                          return false;
                      }
                      return true;
                  });
                  
                  console.log('Valid MCDs after filtering:', validFeatures.length);
                  return validFeatures;
              } catch (error) {
                  console.error('Error fetching mesoscale discussions:', error);
                  return [];
              }
          }
          
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
          
          // Clear all warning layers from the map
          function clearWarningLayers() {
              warningLayers.forEach(layer => {
                  if (map.hasLayer(layer)) {
                      map.removeLayer(layer);
                  }
              });
              warningLayers = [];
          }
          
          // Fetch updated warnings data
          async function fetchUpdatedWarnings() {
              try {
                  // Fetch the JSON data file (assumes it's in the same directory as the HTML)
                  const response = await fetch(window.location.href.replace('.html', '.json') + '?t=' + Date.now());
                  if (!response.ok) {
                      throw new Error('Failed to fetch updates');
                  }
                  
                  const data = await response.json();
                  
                  // Check if data is newer than what we have
                  if (data.updatedAtUTC && data.updatedAtUTC * 1000 > lastUpdateTime) {
                      console.log('New data available, updating...');
                      lastUpdateTime = data.updatedAtUTC * 1000;
                      
                      // Update warnings data
                      warningsData = data.warnings;
                      
                      // Update the statistics
                      updateStats(data);
                      
                      // Fetch updated mesoscale discussions
                      mesoscaleDiscussions = await fetchMesoscaleDiscussions();
                      
                      // Clear old map layers and redraw
                      clearWarningLayers();
                      addMesoscaleDiscussionsToMap();
                      addMesoscaleDiscussionsToList();
                      addWarningsToMap();
                      
                      // Update the list view
                      updateListView(data.warnings);
                      
                      console.log('Update complete');
                  } else {
                      console.log('No new data available');
                  }
              } catch (error) {
                  console.error('Error fetching updates:', error);
              }
          }
          
          // Update statistics display
          function updateStats(data) {
              // Update last updated time with local timezone
              if (data.updatedAtUTC) {
                  updateLastUpdatedTime(data.updatedAtUTC);
              }
              
              // Update total warnings count
              const statsElements = document.querySelectorAll('h4');
              statsElements.forEach(el => {
                  if (el.textContent.includes('Total Warnings:')) {
                      el.textContent = 'Total Warnings: ' + data.counter;
                  }
              });
          }
          
          // Add mesoscale discussions to the list view
          function addMesoscaleDiscussionsToList() {
              const container = document.getElementById('mcd-container');
              if (!container) {
                  console.log('MCD container not found');
                  return;
              }
              
              if (mesoscaleDiscussions.length === 0) {
                  container.innerHTML = '';
                  return;
              }
              
              // Create header for MCDs
              let html = '<div class="warning header mcd-header" style="background-color: var(--mcd-bg); border-color: var(--mcd-border); grid-column: 1 / -1; margin-top: 15px; margin-bottom: 5px; padding: 2px 10px; border-width: 2px;">' +
                  '<h2 style="margin: 5px 0; text-align: center; font-size: 1.2em;">Mesoscale Discussions</h2>' +
                  '</div>';
              
              // Add container for MCD cards
              html += '<div class="warnings-container">';
              
              mesoscaleDiscussions.forEach((mcd, index) => {
                  const props = mcd.properties || {};
                  const mcdNumber = props.name || props.prod_id || props.PROD_ID || props.disc_num || props.DISC_NUM || ('MCD-' + (index + 1));
                  const mcdInfo = props.popupinfo || props.label || props.LABEL || props.discussion || 'Mesoscale discussion area';
                  const mcdId = 'mcd-' + index;
                  
                  // Get issue and expiration times if available
                  const issuedTime = props.issuance || props.issued || props.valid_time || '';
                  const expiresTime = props.expiration || props.expires || props.expire_time || '';
                  
                  html += '<div class="warning mcd" data-mcd-id="' + mcdId + '">' +
                      '<div class="warning-header">' +
                      '<h2 class="warning-title" style="cursor: pointer;" onclick="zoomToMCD(' + index + ')">Mesoscale Discussion ' + mcdNumber + '</h2>' +
                      '<strong>SPC MCD</strong>' +
                      '</div>' +
                      '<p>' + mcdInfo + '</p>';
                  
                  if (issuedTime) {
                      html += '<small>Issued: ' + formatTime(issuedTime) + '</small><br>';
                  }
                  if (expiresTime) {
                      html += '<small>Expires: ' + formatTime(expiresTime) + '</small>';
                  }
                  
                  html += '</div>';
              });
              
              html += '</div>';
              container.innerHTML = html;
              
              console.log('Added', mesoscaleDiscussions.length, 'MCDs to list view');
          }
          
          // Zoom to a specific MCD on the map
          function zoomToMCD(index) {
              // Scroll to map first
              document.getElementById('map').scrollIntoView({ behavior: 'smooth', block: 'center' });
              
              const mcd = mesoscaleDiscussions[index];
              if (!mcd || !mcd.geometry) {
                  console.log('MCD not found or has no geometry');
                  return;
              }
              
              try {
                  // Calculate bounds from geometry
                  let bounds;
                  if (mcd.geometry.type === 'Polygon') {
                      const coords = mcd.geometry.coordinates[0];
                      const latLngs = coords.map(coord => [coord[1], coord[0]]);
                      bounds = L.latLngBounds(latLngs);
                  } else if (mcd.geometry.type === 'MultiPolygon') {
                      const coords = mcd.geometry.coordinates[0][0];
                      const latLngs = coords.map(coord => [coord[1], coord[0]]);
                      bounds = L.latLngBounds(latLngs);
                  }
                  
                  if (bounds) {
                      map.fitBounds(bounds, { padding: [50, 50] });
                      console.log('Zoomed to MCD', index);
                  }
              } catch (error) {
                  console.error('Error zooming to MCD:', error);
              }
          }
          
          // Update the list view with new warnings
          function updateListView(warnings) {
              // This is a simplified update - in a full implementation you might want to
              // preserve scroll position, open popups, etc.
              // For now, we'll just let the user manually refresh the page if they want the full list rebuilt
              console.log('List view update - ' + warnings.length + ' warnings');
              // The list will be fully rebuilt on next page load
              // We're primarily updating the map here for real-time tracking
          }
          
          // Format timestamp to user's local time
          function formatLocalTime(timestamp) {
              const date = new Date(timestamp * 1000); // Convert Unix timestamp to milliseconds
              
              // Format with user's locale and timezone
              const options = {
                  year: 'numeric',
                  month: 'short',
                  day: 'numeric',
                  hour: 'numeric',
                  minute: '2-digit',
                  second: '2-digit',
                  timeZoneName: 'short'
              };
              
              return date.toLocaleString(undefined, options);
          }
          
          // Update the last updated time display
          function updateLastUpdatedTime(timestamp) {
              const timeElement = document.getElementById('last-updated-time');
              if (timeElement && timestamp) {
                  timeElement.textContent = formatLocalTime(timestamp);
              }
          }
          
          // Display countdown to next refresh
          window.onload = function() {
              // Initialize map immediately since it's always visible
              initMap();
              
              // Update the last updated time to local timezone on initial load
              const initialTimestamp = {{ .UpdatedAtUTC }};
              if (initialTimestamp) {
                  updateLastUpdatedTime(initialTimestamp);
              }
              
              // Set up refresh countdown
              let refreshTime = 30; // 30 seconds
              const countdownElements = document.querySelectorAll('.countdown');
              
              const countdownTimer = setInterval(function() {
                  refreshTime--;
                  const minutes = Math.floor(refreshTime / 60);
                  const seconds = refreshTime % 60;
                  const timeString = minutes + ':' + (seconds < 10 ? '0' : '') + seconds;
                  
                  countdownElements.forEach(function(element) {
                      element.textContent = timeString;
                  });
                  
                  if (refreshTime <= 0) {
                      countdownElements.forEach(function(element) {
                          element.textContent = "Updating...";
                      });
                      refreshTime = 30; // Reset for next cycle
                  }
              }, 1000);
              
              // Start periodic updates (every 30 seconds)
              setInterval(fetchUpdatedWarnings, updateInterval);
              
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
          
          // Zoom to a specific warning on the map
          function zoomToWarning(warningId) {
              // Scroll to map first
              document.getElementById('map').scrollIntoView({ behavior: 'smooth', block: 'center' });
              
              // Find the warning in our data
              const warning = warningsData.find(w => w.id === warningId);
              if (!warning) {
                  console.log('Warning not found:', warningId);
                  return;
              }
              
              // Find the corresponding polygon layer
              const layer = warningLayers.find(layer => {
                  // Check if this layer corresponds to our warning
                  if (layer._popup && layer._popup._content) {
                      return layer._popup._content.includes(warning.type) && 
                             layer._popup._content.includes(warning.area);
                  }
                  return false;
              });
              
              if (layer && layer.getBounds) {
                  // Zoom to the polygon
                  map.fitBounds(layer.getBounds(), { padding: [50, 50] });
                  
                  // Open the popup after a short delay
                  setTimeout(function() {
                      layer.openPopup();
                  }, 500);
              } else if (warning.geometry && warning.geometry.coordinates) {
                  // Fallback: calculate bounds from coordinates
                  try {
                      const coords = warning.geometry.type === 'Polygon' 
                          ? warning.geometry.coordinates[0] 
                          : warning.geometry.coordinates[0][0];
                      
                      const latLngs = coords.map(coord => [coord[1], coord[0]]);
                      const bounds = L.latLngBounds(latLngs);
                      map.fitBounds(bounds, { padding: [50, 50] });
                  } catch (error) {
                      console.error('Error zooming to warning:', error);
                  }
              }
          }
          
          // Initialize the map
          async function initMap() {
              // Create map centered on continental US
              map = L.map('map').setView([39.8283, -98.5795], 4);
              
              // Restore saved map position if available
              const savedMapState = localStorage.getItem('mapState');
              if (savedMapState) {
                  try {
                      const state = JSON.parse(savedMapState);
                      map.setView([state.lat, state.lng], state.zoom);
                  } catch (e) {
                      console.log('Could not restore map state:', e);
                  }
              }
              
              // Add OpenStreetMap tile layer with dark theme
              L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
                  attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
                  subdomains: 'abcd',
                  maxZoom: 20
              }).addTo(map);
              
              // Save map state whenever user moves or zooms
              map.on('moveend', saveMapState);
              map.on('zoomend', saveMapState);
              
              // Load county boundaries if not already loaded
              if (!countyBoundaries) {
                  await loadCountyBoundaries();
              }
              
              // Fetch and add mesoscale discussions
              mesoscaleDiscussions = await fetchMesoscaleDiscussions();
              addMesoscaleDiscussionsToMap();
              addMesoscaleDiscussionsToList();
              
              // Add warnings to map
              addWarningsToMap();
          }
          
          // Save current map state to localStorage
          function saveMapState() {
              const center = map.getCenter();
              const zoom = map.getZoom();
              const state = {
                  lat: center.lat,
                  lng: center.lng,
                  zoom: zoom
              };
              localStorage.setItem('mapState', JSON.stringify(state));
          }
          
          // Add mesoscale discussions to the map
          function addMesoscaleDiscussionsToMap() {
              console.log('Adding MCDs to map. Total MCDs:', mesoscaleDiscussions.length);
              
              if (mesoscaleDiscussions.length === 0) {
                  console.log('No mesoscale discussions to display');
                  return;
              }
              
              mesoscaleDiscussions.forEach((mcd, index) => {
                  console.log('Processing MCD', index + 1, ':', mcd);
                  console.log('Full MCD object:', JSON.stringify(mcd, null, 2));
                  
                  if (!mcd.geometry) {
                      console.log('MCD', index + 1, 'has no geometry, skipping');
                      return;
                  }
                  
                  try {
                      console.log('MCD geometry:', JSON.stringify(mcd.geometry, null, 2));
                      console.log('MCD geometry type:', mcd.geometry.type);
                      console.log('MCD geometry coordinates:', mcd.geometry.coordinates);
                      console.log('MCD properties:', JSON.stringify(mcd.properties, null, 2));
                      
                      // Add MCD polygon to map with distinct styling
                      const geoJsonLayer = L.geoJSON(mcd, {
                          style: {
                              color: '#00aaaa',
                              fillColor: '#00aaaa',
                              fillOpacity: 0.3,
                              weight: 3,
                              opacity: 1.0,
                              dashArray: '10, 5'
                          }
                      }).addTo(map);
                      
                      console.log('Created geoJSON layer:', geoJsonLayer);
                      console.log('Layer bounds:', geoJsonLayer.getBounds ? geoJsonLayer.getBounds() : 'No bounds');
                      
                      // Track this layer
                      warningLayers.push(geoJsonLayer);
                      
                      // Get MCD properties - try all possible field names
                      const props = mcd.properties || {};
                      console.log('All property keys:', Object.keys(props));
                      const mcdNumber = props.name || props.prod_id || props.PROD_ID || props.disc_num || props.DISC_NUM || 'Unknown';
                      const mcdInfo = props.popupinfo || props.label || props.LABEL || props.discussion || 'No details available';
                      
                      console.log('MCD Number:', mcdNumber, 'Info:', mcdInfo);
                      
                      // Create popup
                      const popupContent = '<div style="color: #000; min-width: 250px; max-width: 400px;">' +
                          '<h3 style="margin-top: 0;">Mesoscale Discussion ' + mcdNumber + '</h3>' +
                          '<p><strong>Type:</strong> SPC Mesoscale Discussion</p>' +
                          '<div style="margin-top: 10px; font-size: 0.9em;">' + mcdInfo + '</div>' +
                          '<div style="margin-top: 10px; font-size: 0.8em; color: #666;">' +
                          'All properties: ' + JSON.stringify(props) +
                          '</div>' +
                          '</div>';
                      
                      geoJsonLayer.bindPopup(popupContent, {
                          maxWidth: 400,
                          maxHeight: 400
                      });
                      
                      // Add tooltip
                      geoJsonLayer.bindTooltip('MCD ' + mcdNumber, {
                          sticky: true
                      });
                      
                      console.log('Successfully added MCD:', mcdNumber);
                      
                      // Try to zoom to the MCD to see if it's there
                      if (geoJsonLayer.getBounds) {
                          console.log('MCD bounds:', geoJsonLayer.getBounds());
                      }
                  } catch (error) {
                      console.error('Error adding MCD', index + 1, 'to map:', error);
                      console.error('Error stack:', error.stack);
                  }
              });
              
              console.log('Finished adding', mesoscaleDiscussions.length, 'mesoscale discussions to map');
          }
          
          // Get color based on warning type
          function getWarningColor(warningType, severity) {
              const lowerType = warningType ? warningType.toLowerCase() : '';
              
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
              
              console.log('Total warnings data:', warningsData.length);
              
              // Filter out warnings without geometry first
              const validWarnings = warningsData.filter(warning => {
                  // Skip header entries or undefined warnings
                  if (!warning || warning.severity === 'Header') return false;
                  
                  // Must have geometry or SAME codes for county fallback
                  return (warning.geometry && warning.geometry.type) || 
                         (warning.same && warning.same.length > 0 && countyBoundaries);
              });
              
              console.log('Valid warnings to display:', validWarnings.length);
              
              // Sort warnings by priority (similar to alerts.js order)
              const warningOrder = [
                  'tornado warning',
                  'severe thunderstorm warning',
                  'tornado watch',
                  'severe thunderstorm watch',
                  'other'
              ];
              
              validWarnings.sort((a, b) => {
                  const aType = (a.type || '').toLowerCase();
                  const bType = (b.type || '').toLowerCase();
                  
                  let aIndex = warningOrder.length - 1; // default to 'other'
                  let bIndex = warningOrder.length - 1;
                  
                  for (let i = 0; i < warningOrder.length - 1; i++) {
                      if (aType.includes(warningOrder[i])) aIndex = i;
                      if (bType.includes(warningOrder[i])) bIndex = i;
                  }
                  
                  if (aIndex !== bIndex) {
                      return aIndex - bIndex;
                  }
                  
                  // Sort by time if same type
                  const aTime = new Date(a.time || 0).getTime();
                  const bTime = new Date(b.time || 0).getTime();
                  return aTime - bTime;
              });
              
              // Draw warnings
              validWarnings.forEach(warning => {
                  const color = getWarningColor(warning.type, warning.severity);
                  
                  try {
                      // Try to use actual polygon geometry first
                      if (warning.geometry && warning.geometry.type) {
                          const geometry = warning.geometry;
                          
                          if (geometry.type === 'Polygon') {
                              drawPolygon(geometry.coordinates, warning, color);
                              addedCount++;
                          } else if (geometry.type === 'MultiPolygon') {
                              geometry.coordinates.forEach(polygonCoords => {
                                  drawPolygon(polygonCoords, warning, color);
                              });
                              addedCount++;
                          } else {
                              console.log('Unknown geometry type:', geometry.type, 'for warning:', warning.type);
                              // Try county fallback
                              if (addCountyFallback(warning, color)) {
                                  countyFallbackCount++;
                                  addedCount++;
                              } else {
                                  skippedCount++;
                              }
                          }
                      } 
                      // Fallback to county boundaries if no geometry
                      else if (warning.same && warning.same.length > 0 && countyBoundaries) {
                          if (addCountyFallback(warning, color)) {
                              countyFallbackCount++;
                              addedCount++;
                          } else {
                              console.log('Warning missing geometry and county fallback failed:', warning.type, warning.area);
                              skippedCount++;
                          }
                      }
                  } catch (error) {
                      console.error('Error adding warning to map:', warning.type, error);
                      skippedCount++;
                  }
              });
              
              console.log('Map loaded:', addedCount, 'warnings added,', skippedCount, 'skipped');
              if (countyFallbackCount > 0) {
                  console.log('County fallback used for', countyFallbackCount, 'warnings');
              }
          }
          
          // Draw a single polygon on the map
          function drawPolygon(coordinates, warning, color) {
              try {
                  // Convert coordinates from [lng, lat] to [lat, lng] for Leaflet
                  const latLngs = coordinates[0].map(coord => [coord[1], coord[0]]);
                  
                  // Create polygon with styling based on warning type
                  const polygon = L.polygon(latLngs, {
                      color: color,
                      fillColor: color,
                      fillOpacity: 0.3,
                      weight: 2,
                      opacity: 0.8
                  }).addTo(map);
                  
                  // Track this layer for cleanup
                  warningLayers.push(polygon);
                  
                  // Create popup content with description
                  const popupContent = '<div style="color: #000; min-width: 250px; max-width: 400px;">' +
                      '<h3 style="margin-top: 0;">' + (warning.type || 'Unknown') + '</h3>' +
                      '<p><strong>Severity:</strong> ' + (warning.severity || 'Unknown') + '</p>' +
                      '<p><strong>Area:</strong> ' + (warning.area || 'Unknown') + '</p>' +
                      '<p><strong>Issued:</strong> ' + formatTime(warning.time) + '</p>' +
                      '<p><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                      (warning.description ? '<div style="margin-top: 10px; padding-top: 10px; border-top: 1px solid #ccc;"><strong>Details:</strong><p style="margin-top: 5px; font-size: 0.9em; max-height: 200px; overflow-y: auto;">' + warning.description + '</p></div>' : '') +
                      '</div>';
                  
                  polygon.bindPopup(popupContent, {
                      maxWidth: 400,
                      maxHeight: 400
                  });
                  
                  // Add tooltip on hover
                  polygon.bindTooltip((warning.type || 'Warning') + ' - ' + (warning.area || 'Unknown area'), {
                      sticky: true
                  });
                  
                  console.log('Drew polygon for:', warning.type, 'in', warning.area);
              } catch (error) {
                  console.error('Error creating polygon for', warning.type, ':', error);
                  throw error;
              }
          }
          
          // Add county boundaries as fallback when specific polygon is not available
          function addCountyFallback(warning, color) {
              if (!countyBoundaries || !warning.same || warning.same.length === 0) {
                  return false;
              }
              
              let addedAny = false;
              
              // Convert SAME codes to FIPS format and add county boundaries
              warning.same.forEach(sameCode => {
                  // SAME code format: first 3 digits are state FIPS (with leading 0), last 3 are county FIPS
                  // We need it as 5 digits total for matching
                  const fipsCode = sameCode.substring(1); // Remove leading 0 to get standard 5-digit FIPS
                  
                  // Find matching county in the GeoJSON
                  const county = countyBoundaries.features.find(feature => 
                      feature.id === fipsCode || feature.properties.GEO_ID === fipsCode
                  );
                  
                  if (county && county.geometry) {
                      try {
                          // Add the county boundary to the map with distinct styling
                          const geoJsonLayer = L.geoJSON(county, {
                              style: {
                                  color: color,
                                  fillColor: color,
                                  fillOpacity: 0.15, // Lighter opacity for county fallback
                                  weight: 2,
                                  opacity: 0.6,
                                  dashArray: '5, 5' // Dashed line to indicate it's a county boundary
                              }
                          }).addTo(map);
                          
                          // Track this layer for cleanup
                          warningLayers.push(geoJsonLayer);
                          
                          // Create popup content with description
                          const popupContent = '<div style="color: #000; min-width: 250px; max-width: 400px;">' +
                              '<h3 style="margin-top: 0;">' + (warning.type || 'Unknown') + '</h3>' +
                              '<p><strong>Severity:</strong> ' + (warning.severity || 'Unknown') + '</p>' +
                              '<p><strong>Area:</strong> ' + (warning.area || 'Unknown') + '</p>' +
                              '<p><strong>Issued:</strong> ' + formatTime(warning.time) + '</p>' +
                              '<p><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                              (warning.description ? '<div style="margin-top: 10px; padding-top: 10px; border-top: 1px solid #ccc;"><strong>Details:</strong><p style="margin-top: 5px; font-size: 0.9em; max-height: 200px; overflow-y: auto;">' + warning.description + '</p></div>' : '') +
                              '<p style="font-style: italic; font-size: 0.85em; margin-top: 10px; padding-top: 10px; border-top: 1px solid #ccc;">' +
                              '⚠️ Showing county boundary - exact warning polygon unavailable</p>' +
                              '</div>';
                          
                          geoJsonLayer.bindPopup(popupContent, {
                              maxWidth: 400,
                              maxHeight: 400
                          });
                          
                          // Add tooltip
                          geoJsonLayer.bindTooltip((warning.type || 'Warning') + ' - ' + (warning.area || 'Unknown area') + ' (County)', {
                              sticky: true
                          });
                          
                          addedAny = true;
                          console.log('Added county fallback for:', warning.type, 'FIPS:', fipsCode);
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
                      info += (index + 1) + '. ' + (warning.type || 'Unknown') + ' - ' + (warning.area || 'Unknown') + '\n';
                      info += '   Severity: ' + (warning.severity || 'Unknown') + '\n';
                      info += '   Has Geometry: ' + (warning.geometry ? 'Yes' : 'NO') + '\n';
                      if (warning.geometry) {
                          info += '   Geometry Type: ' + (warning.geometry.type || 'unknown') + '\n';
                      }
                      if (warning.same && warning.same.length > 0) {
                          info += '   SAME Codes: ' + warning.same.join(', ') + '\n';
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
       <h4 id="last-updated">Last updated: <span id="last-updated-time">{{ .LastUpdated }}</span></h4>
       <div class="next-refresh">Next refresh in <span class="countdown">0:30</span></div>
       
       <!-- Map Section -->
       <div id="map-section" style="margin-bottom: 30px;">
          <h2 style="margin-top: 20px;">Map View</h2>
          <button onclick="showDebugInfo()" style="margin-bottom: 10px; padding: 5px 10px; background-color: var(--header-bg); color: var(--text-color); border: 1px solid var(--header-border); border-radius: 3px; cursor: pointer;">Show Debug Info</button>
          <div id="debug-info" style="display: none; background-color: var(--summary-bg); padding: 10px; margin-bottom: 10px; border-radius: 5px; font-family: monospace; font-size: 0.9em; max-height: 200px; overflow-y: auto;"></div>
          <div id="map"></div>
       </div>
       
       <!-- List Section -->
       <div id="list-section">
          <h2 style="margin-top: 20px;">List View</h2>
          
          <!-- Mesoscale Discussions Container (populated by JavaScript) -->
          <div id="mcd-container"></div>
          
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
                   <div class="warning {{ .SeverityClass }}" data-warning-id="{{ .ID }}">
                      <div class="warning-header">
                         <h2 class="warning-title" style="cursor: pointer;" onclick="zoomToWarning('{{ .ID }}')">{{ .Type }}</h2>
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
       
       <!-- Map Legend -->
       <div class="map-legend" style="margin-top: 30px;">
          <h3>Map Legend</h3>
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
             <div class="legend-color" style="background-color: #00aaaa;"></div>
             <span>Mesoscale Discussion (MCD)</span>
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
             <strong>Note:</strong> Solid polygons show exact warning boundaries. Dashed lines indicate county boundaries (used when exact polygon unavailable) or mesoscale discussions.
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
		UpdatedAtUTC      int64
	}{
		Warnings:          convertWarnings(warnings),
		LastUpdated:       time.Now().UTC().Format("Jan 2, 2006 at 03:04:01 UTC"),
		Counter:           len(warnings),
		WarningTypeCounts: sortedWarningTypeCounts(warnings),
		WarningsJSON:      template.JS(warningsJSON),
		UpdatedAtUTC:      time.Now().UTC().Unix(),
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
	ID               string // Warning ID for linking to map
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
			ID:               warning.ID,
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

// GenerateWarningsJSON creates a JSON file with current warnings data for AJAX updates
func GenerateWarningsJSON(warnings []fetcher.Warning, outputPath string) error {
	// Create a data structure for the JSON
	data := struct {
		Warnings     []fetcher.Warning `json:"warnings"`
		LastUpdated  string            `json:"lastUpdated"`
		Counter      int               `json:"counter"`
		UpdatedAtUTC int64             `json:"updatedAtUTC"`
	}{
		Warnings:     warnings,
		LastUpdated:  time.Now().UTC().Format("Jan 2, 2006 at 03:04:01 UTC"),
		Counter:      len(warnings),
		UpdatedAtUTC: time.Now().UTC().Unix(),
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	return os.WriteFile(outputPath, jsonData, 0644)
}
