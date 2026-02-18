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
             animation: fadeIn 0.3s ease-in;
          }
          
          @keyframes fadeIn {
             from { opacity: 0; }
             to { opacity: 1; }
          }
          
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
             grid-column: 1 / -1;
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
          .warning.header.mcd-header {
             background-color: var(--mcd-bg);
             border-color: var(--mcd-border);
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
           .warning-type.moderate {
              background-color: #4d3510;
              border: 1px solid #b25900;
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
          .mcd-description {
             font-size: 0.85em;
             color: #aaa;
             margin-top: 8px;
             max-height: 80px;
             overflow-y: auto;
             border-top: 1px solid var(--mcd-border);
             padding-top: 6px;
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
          let countyBoundaries = null;
          let warningLayers = [];
          let mesoscaleDiscussions = [];
          let updateInterval = 30000;
          let lastUpdateTime = Date.now();
          
          // ---------------------------------------------------------------
          // Fetch Mesoscale Discussions from the NOAA ArcGIS MapServer,
          // then scrape the full text from each SPC product page via the
          // allorigins CORS proxy.
          //
          // Key fix: ArcGIS returns 3D coordinates [lng, lat, z]. Leaflet
          // only handles 2D, so we strip the Z value before drawing.
          // ---------------------------------------------------------------
          async function fetchMesoscaleDiscussions() {
              try {
                  // Step 1 — get active MCD polygons + metadata from NOAA MapServer
                  const base = 'https://mapservices.weather.noaa.gov/vector/rest/services/outlooks/spc_mesoscale_discussion/MapServer/0/query';
                  const params = new URLSearchParams({
                      where:     '1=1',
                      outFields: 'name,folderpath,popupinfo,idp_filedate',
                      f:         'geojson',
                      _:         Date.now()
                  });

                  const response = await fetch(base + '?' + params);
                  if (!response.ok) throw new Error('SPC MapServer fetch failed: ' + response.status);

                  const data     = await response.json();
                  const features = (data.features || []).filter(f => f.geometry && f.geometry.coordinates);
                  console.log('SPC MapServer MCDs (active):', features.length);

                  // Strip Z coordinate from all geometries so Leaflet can draw them.
                  // ArcGIS returns [lng, lat, z] triplets; Leaflet needs [lng, lat] pairs.
                  features.forEach(feature => {
                      feature.geometry = stripZ(feature.geometry);
                  });

                  // Step 2 — scrape full discussion text from each SPC HTML page in parallel
                  await Promise.all(features.map(async (feature) => {
                      const props  = feature.properties || {};
                      const mcdNum = extractMCDNumber(props);
                      const spcUrl = mcdSPCLink(mcdNum);
                      try {
                          const proxyUrl = 'https://corsproxy.io/?' + encodeURIComponent(spcUrl);
                          const r = await fetch(proxyUrl, { signal: AbortSignal.timeout(10000) });
                          if (!r.ok) throw new Error('proxy HTTP ' + r.status);

                          const html  = await r.text();
                          // SPC page has exactly one <pre> containing the raw discussion text
                          const preMatch = html.match(/<pre[^>]*>([\s\S]*?)<\/pre>/i);
                          if (preMatch) {
                              feature.properties._fullText = preMatch[1]
                                  .replace(/<[^>]+>/g, '')
                                  .replace(/&amp;/g,  '&')
                                  .replace(/&lt;/g,   '<')
                                  .replace(/&gt;/g,   '>')
                                  .replace(/&nbsp;/g, ' ')
                                  .trim();
                              console.log('MCD #' + mcdNum + ' text scraped (' + feature.properties._fullText.length + ' chars)');
                          } else {
                              console.warn('MCD #' + mcdNum + ': no <pre> tag found in SPC page');
                          }
                      } catch (e) {
                          console.warn('MCD #' + mcdNum + ' scrape failed:', e.message);
                      }
                  }));

                  return features;

              } catch (error) {
                  console.error('Error fetching SPC MCDs:', error);
                  return [];
              }
          }

          // Strip Z (elevation) values from ArcGIS GeoJSON geometry.
          // ArcGIS encodes coordinates as [lng, lat, z]; Leaflet needs [lng, lat].
          function stripZ(geometry) {
              if (!geometry) return geometry;
              const strip2D = coords => coords.map(c => [c[0], c[1]]);
              switch (geometry.type) {
                  case 'Polygon':
                      return { type: 'Polygon', coordinates: geometry.coordinates.map(strip2D) };
                  case 'MultiPolygon':
                      return { type: 'MultiPolygon', coordinates: geometry.coordinates.map(ring => ring.map(strip2D)) };
                  default:
                      return geometry;
              }
          }

          // The 'name' field from the MapServer IS the MCD number (e.g. "421" or "0421").
          // The MapServer 'name' field may come back as "MD 0091" or "0091".
          // Strip any non-digit characters before zero-padding to get a clean number.
          function extractMCDNumber(props) {
              if (!props.name) return '????';
              const digits = String(props.name).replace(/[^0-9]/g, '');
              return digits ? digits.padStart(4, '0') : '????';
          }

          // ArcGIS dates are Unix milliseconds; convert to a locale string.
          function formatArcGISDate(msTimestamp) {
              if (!msTimestamp && msTimestamp !== 0) return 'Not specified';
              return new Date(msTimestamp).toLocaleString();
          }

          // Build the SPC product page URL for a given zero-padded MCD number.
          function mcdSPCLink(mcdNum) {
              const year = new Date().getFullYear();
              return 'https://www.spc.noaa.gov/products/md/' + year + '/md' + mcdNum + '.html';
          }

          // Parse the raw SPC <pre> text into labelled sections.
          //
          // SPC MCD text format:
          //   MESOSCALE DISCUSSION 0421
          //   NWS STORM PREDICTION CENTER NORMAN OK
          //   ...
          //   AREA AFFECTED...PORTIONS OF CENTRAL TX AND SOUTHWEST OK
          //   CONCERNING...SEVERE THUNDERSTORM WATCH ISSUANCE
          //   VALID 172015Z - 172215Z
          //
          //   SUMMARY...
          //   Thunderstorm activity is expected to increase...
          //
          //   DISCUSSION...
          //   A well-defined shortwave trough...
          //
          //   ..Forecaster Name.. DD/HHMM UTC
          function parseMCDText(rawText) {
              if (!rawText) return { area: '', concerning: '', valid: '', summary: '', discussion: '', raw: '' };

              const startIdx = rawText.indexOf('MESOSCALE DISCUSSION');
              const text = startIdx >= 0 ? rawText.substring(startIdx) : rawText;

              // Grab single-line value after LABEL...
              const getInline = (label) => {
                  const re = new RegExp(label + '\\.{3}(.+)', 'i');
                  const m  = text.match(re);
                  return m ? m[1].trim() : '';
              };

              // Grab multi-line block after LABEL... until next ALL-CAPS header or ..sig
              const getBlock = (label) => {
                  const re = new RegExp(label + '\\.{3}[\\s\\S]*?(?=\\n[A-Z][A-Z .]{2,}\\.{3}|\\n\\.\\.|[A-Z][A-Z]+\\.{3}|\\nMESOSCALE|$)', 'i');
                  const m  = text.match(re);
                  if (!m) return '';
                  let content = m[0];
                  content = content.replace(new RegExp('^' + label + '\\.{3}', 'i'), '');
                  return content.replace(/\s+/g, ' ').trim();
              };

              return {
                  area:       getInline('AREA AFFECTED'),
                  concerning: getInline('CONCERNING'),
                  valid:      getInline('VALID'),
                  summary:    getBlock('SUMMARY'),
                  discussion: getBlock('DISCUSSION'),
                  raw:        text
              };
          }
          
          // Load county boundaries from NWS GeoJSON
          async function loadCountyBoundaries() {
              try {
                  const response = await fetch('https://raw.githubusercontent.com/plotly/datasets/master/geojson-counties-fips.json');
                  countyBoundaries = await response.json();
                  console.log('County boundaries loaded successfully');
              } catch (error) {
                  console.error('Failed to load county boundaries:', error);
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
                  const response = await fetch(window.location.href.replace('.html', '.json') + '?t=' + Date.now());
                  if (!response.ok) throw new Error('Failed to fetch updates');
                  
                  const data = await response.json();
                  
                  if (data.updatedAtUTC && data.updatedAtUTC * 1000 > lastUpdateTime) {
                      console.log('New data available, updating...');
                      lastUpdateTime = data.updatedAtUTC * 1000;
                      warningsData = data.warnings;
                      updateStats(data);

                      // Redraw NWS warning polygons immediately
                      clearWarningLayers();
                      addWarningsToMap();
                      updateListView(data.warnings);

                      // Reload MCDs independently in background
                      fetchMesoscaleDiscussions().then(features => {
                          mesoscaleDiscussions = features;
                          addMesoscaleDiscussionsToMap();
                          addMesoscaleDiscussionsToList();
                          enrichMCDsWithText();
                      });

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
              if (data.updatedAtUTC) updateLastUpdatedTime(data.updatedAtUTC);
              const statsElements = document.querySelectorAll('h4');
              statsElements.forEach(el => {
                  if (el.textContent.includes('Total Warnings:')) {
                      el.textContent = 'Total Warnings: ' + data.counter;
                  }
              });
          }
          
          // ------------------------------------------------------------------
          // Render MCD cards into the #mcd-container list section.
          // Shows: MCD number, issued time, area, concerning, valid period,
          // summary text, and a link to the full SPC discussion.
          // ------------------------------------------------------------------
          function addMesoscaleDiscussionsToList() {
              const container = document.getElementById('mcd-container');
              if (!container) return;
              
              // Update the MCD count in the header
              const mcdCountEl = document.getElementById('mcd-count');
              if (mcdCountEl) mcdCountEl.textContent = mesoscaleDiscussions.length;
              
              if (mesoscaleDiscussions.length === 0) {
                  container.innerHTML = '';
                  return;
              }
              
              let html = '<div class="warning header mcd-header" style="grid-column: 1 / -1; margin-top: 20px;">' +
                  '<h2>Mesoscale Discussions (' + mesoscaleDiscussions.length + ')</h2>' +
                  '</div>' +
                  '<div class="warnings-container">';
              
              mesoscaleDiscussions.forEach((mcd, index) => {
                  const props   = mcd.properties || {};
                  const mcdNum  = extractMCDNumber(props);
                  const issued  = props.idp_filedate ? formatArcGISDate(props.idp_filedate) : 'Not specified';
                  const spcUrl  = mcdSPCLink(mcdNum);
                  const parsed  = parseMCDText(props._fullText || '');

                  html += '<div class="warning mcd">';
                  html += '<div class="warning-header">' +
                      '<h2 class="warning-title" style="cursor:pointer;" onclick="zoomToMCD(' + index + ')">' +
                          'MCD #' + mcdNum +
                      '</h2>' +
                      '<strong>SPC</strong>' +
                  '</div>';

                  html += '<small><strong>Issued:</strong> ' + escapeHtml(issued) + '</small>';

                  if (parsed.area) {
                      html += '<p style="margin:6px 0 2px;"><strong>Area:</strong> ' + escapeHtml(parsed.area) + '</p>';
                  }
                  if (parsed.concerning) {
                      html += '<p style="margin:2px 0;"><strong>Concerning:</strong> ' + escapeHtml(parsed.concerning) + '</p>';
                  }
                  if (parsed.valid) {
                      html += '<p style="margin:2px 0;"><strong>Valid:</strong> ' + escapeHtml(parsed.valid) + '</p>';
                  }
                  if (parsed.summary) {
                      html += '<div class="mcd-description" style="max-height:120px;">' +
                          '<strong>Summary:</strong> ' + escapeHtml(parsed.summary) +
                      '</div>';
                  }

                  if (parsed.discussion) {
                      html += '<div class="mcd-description" style="max-height:150px; margin-top:6px;">' +
                          '<strong>Discussion:</strong> ' + escapeHtml(parsed.discussion) +
                      '</div>';
                  } else if (parsed.summary) {
                      html += '<div class="mcd-description" style="max-height:120px; margin-top:6px;">' +
                          '<strong>Summary:</strong> ' + escapeHtml(parsed.summary) +
                      '</div>';
                  } else if (parsed.area || parsed.concerning || parsed.valid) {
                      // Has structured fields but no summary/discussion - show what we have
                      if (parsed.area) {
                          html += '<p style="margin:6px 0 2px;"><strong>Area:</strong> ' + escapeHtml(parsed.area) + '</p>';
                      }
                      if (parsed.concerning) {
                          html += '<p style="margin:2px 0;"><strong>Concerning:</strong> ' + escapeHtml(parsed.concerning) + '</p>';
                      }
                      if (parsed.valid) {
                          html += '<p style="margin:2px 0;"><strong>Valid:</strong> ' + escapeHtml(parsed.valid) + '</p>';
                      }
                  } else if (props._fullText) {
                      // No structured fields - show raw full text
                      const plainText = props._fullText.replace(/\s+/g, ' ').trim();
                      if (plainText) {
                          html += '<div class="mcd-description">' +
                              escapeHtml(plainText.substring(0, 500)) + (plainText.length > 500 ? '…' : '') +
                          '</div>';
                      }
                  } else {
                      // No structured text available yet — show raw popupinfo snippet
                      const rawPopup  = props.popupinfo || '';
                      const plainText = rawPopup.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
                      if (plainText) {
                          html += '<div class="mcd-description">' +
                              escapeHtml(plainText.substring(0, 300)) + (plainText.length > 300 ? '…' : '') +
                          '</div>';
                      }
                  }

                  html += '<div style="margin-top:8px;">' +
                      '<small><a href="' + spcUrl + '" target="_blank" style="color:#00aaaa;">View full discussion on SPC ↗</a></small>' +
                  '</div>';
                  html += '</div>';
              });
              
              html += '</div>';
              container.innerHTML = html;
              updateAllExpirationCountdowns();
          }

          // Simple HTML escaper to avoid XSS in dynamic content
          function escapeHtml(str) {
              return String(str)
                  .replace(/&/g, '&amp;')
                  .replace(/</g, '&lt;')
                  .replace(/>/g, '&gt;')
                  .replace(/"/g, '&quot;');
          }
          
          // Zoom to a specific MCD on the map
          function zoomToMCD(index) {
              document.getElementById('map').scrollIntoView({ behavior: 'smooth', block: 'center' });
              
              const mcd = mesoscaleDiscussions[index];
              if (!mcd || !mcd.geometry) {
                  console.log('MCD not found or has no geometry');
                  return;
              }
              
              try {
                  let bounds;
                  if (mcd.geometry.type === 'Polygon') {
                      const coords = mcd.geometry.coordinates[0];
                      bounds = L.latLngBounds(coords.map(c => [c[1], c[0]]));
                  } else if (mcd.geometry.type === 'MultiPolygon') {
                      const allCoords = mcd.geometry.coordinates.flat(1);
                      bounds = L.latLngBounds(allCoords.map(c => [c[1], c[0]]));
                  }
                  
                  if (bounds && bounds.isValid()) {
                      map.fitBounds(bounds, { padding: [50, 50] });
                  }
              } catch (error) {
                  console.error('Error zooming to MCD:', error);
              }
          }
          
          // Update the list view with new warnings
          function updateListView(warnings) {
              console.log('List view update - ' + warnings.length + ' warnings');
          }
          
          // Format timestamp to user's local time
          function formatLocalTime(timestamp) {
              const date = new Date(timestamp * 1000);
              return date.toLocaleString(undefined, {
                  year: 'numeric', month: 'short', day: 'numeric',
                  hour: 'numeric', minute: '2-digit', second: '2-digit',
                  timeZoneName: 'short'
              });
          }
          
          function updateLastUpdatedTime(timestamp) {
              const el = document.getElementById('last-updated-time');
              if (el && timestamp) el.textContent = formatLocalTime(timestamp);
          }
          
          window.onload = function() {
              initMap();
              
              const initialTimestamp = {{ .UpdatedAtUTC }};
              if (initialTimestamp) updateLastUpdatedTime(initialTimestamp);
              
              let refreshTime = 30;
              const countdownElements = document.querySelectorAll('.countdown');
              
              setInterval(function() {
                  refreshTime--;
                  const minutes = Math.floor(refreshTime / 60);
                  const seconds = refreshTime % 60;
                  const timeString = minutes + ':' + (seconds < 10 ? '0' : '') + seconds;
                  countdownElements.forEach(el => el.textContent = timeString);
                  if (refreshTime <= 0) {
                      countdownElements.forEach(el => el.textContent = "Updating...");
                      refreshTime = 30;
                  }
              }, 1000);
              
              setInterval(fetchUpdatedWarnings, updateInterval);
              
              const backToTopButton = document.querySelector('.back-to-top');
              window.addEventListener('scroll', function() {
                  backToTopButton.style.display = window.scrollY > 300 ? 'block' : 'none';
              });
              if (window.scrollY <= 300) backToTopButton.style.display = 'none';
              
              updateAllExpirationCountdowns();
              setInterval(updateAllExpirationCountdowns, 1000);
          }
          
          // Zoom to a specific NWS warning on the map
          function zoomToWarning(warningId) {
              document.getElementById('map').scrollIntoView({ behavior: 'smooth', block: 'center' });
              
              const warning = warningsData.find(w => w.id === warningId);
              if (!warning) return;
              
              const layer = warningLayers.find(layer => {
                  if (layer._popup && layer._popup._content) {
                      return layer._popup._content.includes(warning.type) &&
                             layer._popup._content.includes(warning.area);
                  }
                  return false;
              });
              
              if (layer && layer.getBounds) {
                  map.fitBounds(layer.getBounds(), { padding: [50, 50] });
                  setTimeout(() => layer.openPopup(), 500);
              } else if (warning.geometry && warning.geometry.coordinates) {
                  try {
                      const coords = warning.geometry.type === 'Polygon'
                          ? warning.geometry.coordinates[0]
                          : warning.geometry.coordinates[0][0];
                      map.fitBounds(L.latLngBounds(coords.map(c => [c[1], c[0]])), { padding: [50, 50] });
                  } catch (e) {
                      console.error('Error zooming to warning:', e);
                  }
              }
          }
          
          // Initialize the Leaflet map
          async function initMap() {
              map = L.map('map').setView([39.8283, -98.5795], 4);
              
              const savedMapState = localStorage.getItem('mapState');
              if (savedMapState) {
                  try {
                      const state = JSON.parse(savedMapState);
                      map.setView([state.lat, state.lng], state.zoom);
                  } catch (e) { /* ignore */ }
              }
              
              L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
                  attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
                  subdomains: 'abcd',
                  maxZoom: 20
              }).addTo(map);
              
              map.on('moveend', saveMapState);
              map.on('zoomend', saveMapState);

              // Render NWS warnings immediately — no waiting on anything external
              addWarningsToMap();

              // Load MCDs and county boundaries in parallel, each independent.
              // Neither blocks the other or the warnings already on screen.
              fetchMesoscaleDiscussions().then(features => {
                  mesoscaleDiscussions = features;
                  addMesoscaleDiscussionsToMap();
                  addMesoscaleDiscussionsToList();
                  // Backfill text in background — updates cards/popups in-place as each finishes
                  enrichMCDsWithText();
              });

              loadCountyBoundaries();
          }
          
          function saveMapState() {
              const center = map.getCenter();
              localStorage.setItem('mapState', JSON.stringify({ lat: center.lat, lng: center.lng, zoom: map.getZoom() }));
          }
          
          // ------------------------------------------------------------------
          // Draw all active MCDs on the map using the exact SPC polygon geometry
          // returned by the NOAA ArcGIS MapServer. Popup shows structured
          // discussion text (area, concerning, valid, summary, discussion body).
          // ------------------------------------------------------------------
          function addMesoscaleDiscussionsToMap() {
              if (mesoscaleDiscussions.length === 0) return;
              
              mesoscaleDiscussions.forEach((mcd, index) => {
                  if (!mcd.geometry) return;
                  
                  try {
                      const props  = mcd.properties || {};
                      const mcdNum = extractMCDNumber(props);
                      const issued = props.idp_filedate ? formatArcGISDate(props.idp_filedate) : 'Not specified';
                      const spcUrl = mcdSPCLink(mcdNum);
                      const parsed = parseMCDText(props._fullText || '');

                      const geoJsonLayer = L.geoJSON(mcd, {
                          style: {
                              color: '#00aaaa',
                              fillColor: '#00aaaa',
                              fillOpacity: 0.15,
                              weight: 2,
                              opacity: 0.9,
                              dashArray: '10, 5'
                          }
                      }).addTo(map);
                      
                      warningLayers.push(geoJsonLayer);

                      // Build popup — prefer parsed structured fields, fall back to popupinfo snippet
                      let bodyHtml = '';
                      if (parsed.area || parsed.concerning || parsed.summary || parsed.discussion) {
                          if (parsed.area)      bodyHtml += '<p style="margin:3px 0;"><strong>Area:</strong> '       + escapeHtml(parsed.area)       + '</p>';
                          if (parsed.concerning) bodyHtml += '<p style="margin:3px 0;"><strong>Concerning:</strong> ' + escapeHtml(parsed.concerning) + '</p>';
                          if (parsed.valid)      bodyHtml += '<p style="margin:3px 0;"><strong>Valid:</strong> '      + escapeHtml(parsed.valid)      + '</p>';
                          if (parsed.summary) {
                              bodyHtml += '<div style="margin-top:8px; padding-top:8px; border-top:1px solid #ccc;">' +
                                  '<strong>Summary:</strong>' +
                                  '<p style="margin:4px 0 0; font-size:0.9em; max-height:160px; overflow-y:auto; line-height:1.4;">' +
                                  escapeHtml(parsed.summary) + '</p>' +
                              '</div>';
                          }
                          if (parsed.discussion) {
                              bodyHtml += '<div style="margin-top:8px; padding-top:8px; border-top:1px solid #ccc;">' +
                                  '<strong>Discussion:</strong>' +
                                  '<p style="margin:4px 0 0; font-size:0.85em; max-height:200px; overflow-y:auto; line-height:1.4;">' +
                                  escapeHtml(parsed.discussion) + '</p>' +
                              '</div>';
                          }
                      } else if (props._fullText) {
                          // No structured fields - show raw full text
                          const plainText = props._fullText.replace(/\s+/g, ' ').trim();
                          if (plainText) {
                              bodyHtml = '<p style="font-size:0.9em; max-height:200px; overflow-y:auto;">' +
                                  escapeHtml(plainText.substring(0, 800)) + (plainText.length > 800 ? '…' : '') +
                              '</p>';
                          }
                      } else {
                          // Fallback to popupinfo stripped of HTML tags
                          const rawPopup  = props.popupinfo || '';
                          const plainText = rawPopup.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
                          if (plainText) {
                              bodyHtml = '<p style="font-size:0.9em; max-height:200px; overflow-y:auto;">' +
                                  escapeHtml(plainText.substring(0, 800)) + (plainText.length > 800 ? '…' : '') +
                              '</p>';
                          }
                      }

                      const popupContent =
                          '<div style="color:#000; min-width:260px; max-width:460px;">' +
                          '<h3 style="margin-top:0; margin-bottom:6px; color:#006666;">MCD #' + mcdNum + '</h3>' +
                          '<p style="margin:3px 0;"><strong>Source:</strong> Storm Prediction Center</p>' +
                          '<p style="margin:3px 0;"><strong>Issued:</strong> ' + escapeHtml(issued) + '</p>' +
                          bodyHtml +
                          '<p style="margin-top:10px; padding-top:8px; border-top:1px solid #ccc;">' +
                          '<a href="' + spcUrl + '" target="_blank" style="color:#006666; font-weight:bold;">View full discussion on SPC ↗</a>' +
                          '</p>' +
                          '</div>';
                      
                      geoJsonLayer.bindPopup(popupContent, { maxWidth: 480, maxHeight: 500 });

                      // Tooltip shows a one-liner of what the MCD is about
                      const tooltipText = 'MCD #' + mcdNum +
                          (parsed.concerning ? ' — ' + parsed.concerning : '') +
                          (parsed.area ? ' (' + parsed.area.substring(0, 60) + (parsed.area.length > 60 ? '…' : '') + ')' : '');
                      geoJsonLayer.bindTooltip(tooltipText, { sticky: true });

                      geoJsonLayer.on('mouseover', function(e) {
                          e.target.setStyle({ fillOpacity: 0.3, weight: 3 });
                      });
                      geoJsonLayer.on('mouseout', function(e) {
                          e.target.setStyle({ fillOpacity: 0.15, weight: 2 });
                      });
                      
                  } catch (error) {
                      console.error('Error adding MCD to map:', error);
                  }
              });
          }
          
          // Get color based on warning type
          function getWarningColor(warningType, severity) {
              const lowerType = warningType ? warningType.toLowerCase() : '';
              if (lowerType.includes('tornado warning')) return '#ff69b4';
              if (lowerType.includes('tornado') && lowerType.includes('watch')) return '#ffff00';
              if (lowerType.includes('thunderstorm warning') || lowerType.includes('t-storm warning') || lowerType.includes('tstorm warning')) return '#ff0000';
              if ((lowerType.includes('thunderstorm') || lowerType.includes('t-storm') || lowerType.includes('tstorm')) && lowerType.includes('watch')) return '#aaaa00';
              if (severity === 'Severe' || severity === 'Extreme') return '#a52a2a';
              if (severity === 'Moderate') return '#b25900';
              return '#666666';
          }
          
          // Add NWS warnings to the map
          function addWarningsToMap() {
              let addedCount = 0, skippedCount = 0, countyFallbackCount = 0;
              console.log('Total warnings data:', warningsData.length);
              
              const validWarnings = warningsData.filter(warning => {
                  if (!warning || warning.severity === 'Header') return false;
                  return (warning.geometry && warning.geometry.type) ||
                         (warning.same && warning.same.length > 0 && countyBoundaries);
              });
              
              console.log('Valid warnings to display:', validWarnings.length);
              
              const warningOrder = ['tornado warning','severe thunderstorm warning','tornado watch','severe thunderstorm watch','other'];
              validWarnings.sort((a, b) => {
                  const aType = (a.type || '').toLowerCase();
                  const bType = (b.type || '').toLowerCase();
                  let aIdx = warningOrder.length - 1, bIdx = warningOrder.length - 1;
                  for (let i = 0; i < warningOrder.length - 1; i++) {
                      if (aType.includes(warningOrder[i])) aIdx = i;
                      if (bType.includes(warningOrder[i])) bIdx = i;
                  }
                  if (aIdx !== bIdx) return aIdx - bIdx;
                  return new Date(a.time || 0) - new Date(b.time || 0);
              });
              
              validWarnings.forEach(warning => {
                  const color = getWarningColor(warning.type, warning.severity);
                  try {
                      if (warning.geometry && warning.geometry.type) {
                          if (warning.geometry.type === 'Polygon') {
                              drawPolygon(warning.geometry.coordinates, warning, color);
                              addedCount++;
                          } else if (warning.geometry.type === 'MultiPolygon') {
                              warning.geometry.coordinates.forEach(pc => drawPolygon(pc, warning, color));
                              addedCount++;
                          } else {
                              if (addCountyFallback(warning, color)) { countyFallbackCount++; addedCount++; }
                              else skippedCount++;
                          }
                      } else if (warning.same && warning.same.length > 0 && countyBoundaries) {
                          if (addCountyFallback(warning, color)) { countyFallbackCount++; addedCount++; }
                          else { console.log('County fallback failed:', warning.type, warning.area); skippedCount++; }
                      }
                  } catch (error) {
                      console.error('Error adding warning to map:', warning.type, error);
                      skippedCount++;
                  }
              });
              
              console.log('Map loaded:', addedCount, 'added,', skippedCount, 'skipped,', countyFallbackCount, 'county fallback');
          }
          
          // Draw a single polygon on the map
          function drawPolygon(coordinates, warning, color) {
              const latLngs = coordinates[0].map(coord => [coord[1], coord[0]]);
              const polygon = L.polygon(latLngs, {
                  color, fillColor: color, fillOpacity: 0.3, weight: 2, opacity: 0.8
              }).addTo(map);
              
              warningLayers.push(polygon);
              
              const popupContent = '<div style="color:#000; min-width:250px; max-width:400px;">' +
                  '<h3 style="margin-top:0;">' + (warning.type || 'Unknown') + '</h3>' +
                  '<p><strong>Severity:</strong> ' + (warning.severity || 'Unknown') + '</p>' +
                  '<p><strong>Area:</strong> ' + (warning.area || 'Unknown') + '</p>' +
                  '<p><strong>Issued:</strong> ' + formatTime(warning.time) + '</p>' +
                  '<p><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                  (warning.description ? '<div style="margin-top:10px; padding-top:10px; border-top:1px solid #ccc;"><strong>Details:</strong><p style="font-size:0.9em; max-height:200px; overflow-y:auto;">' + warning.description + '</p></div>' : '') +
                  '</div>';
              
              polygon.bindPopup(popupContent, { maxWidth: 400, maxHeight: 400 });
              polygon.bindTooltip((warning.type || 'Warning') + ' - ' + (warning.area || 'Unknown area'), { sticky: true });
          }
          
          // Add county boundaries as fallback
          function addCountyFallback(warning, color) {
              if (!countyBoundaries || !warning.same || warning.same.length === 0) return false;
              let addedAny = false;
              
              warning.same.forEach(sameCode => {
                  const fipsCode = sameCode.substring(1);
                  const county = countyBoundaries.features.find(f =>
                      f.id === fipsCode || (f.properties && f.properties.GEO_ID === fipsCode)
                  );
                  
                  if (county && county.geometry) {
                      try {
                          const geoJsonLayer = L.geoJSON(county, {
                              style: { color, fillColor: color, fillOpacity: 0.15, weight: 2, opacity: 0.6, dashArray: '5, 5' }
                          }).addTo(map);
                          
                          warningLayers.push(geoJsonLayer);
                          
                          const popupContent = '<div style="color:#000; min-width:250px; max-width:400px;">' +
                              '<h3 style="margin-top:0;">' + (warning.type || 'Unknown') + '</h3>' +
                              '<p><strong>Severity:</strong> ' + (warning.severity || 'Unknown') + '</p>' +
                              '<p><strong>Area:</strong> ' + (warning.area || 'Unknown') + '</p>' +
                              '<p><strong>Issued:</strong> ' + formatTime(warning.time) + '</p>' +
                              '<p><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                              (warning.description ? '<div style="margin-top:10px; padding-top:10px; border-top:1px solid #ccc;"><strong>Details:</strong><p style="font-size:0.9em; max-height:200px; overflow-y:auto;">' + warning.description + '</p></div>' : '') +
                              '<p style="font-style:italic; font-size:0.85em; margin-top:10px; padding-top:10px; border-top:1px solid #ccc;">⚠️ Showing county boundary — exact warning polygon unavailable</p>' +
                              '</div>';
                          
                          geoJsonLayer.bindPopup(popupContent, { maxWidth: 400, maxHeight: 400 });
                          geoJsonLayer.bindTooltip((warning.type || 'Warning') + ' - ' + (warning.area || 'Unknown') + ' (County)', { sticky: true });
                          addedAny = true;
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
              return isNaN(date.getTime()) ? timeStr : date.toLocaleString();
          }
          
          // Show debug information
          function showDebugInfo() {
              const debugDiv = document.getElementById('debug-info');
              if (debugDiv.style.display === 'none') {
                  let info = 'Total warnings: ' + warningsData.length + '\n';
                  info += 'Active MCDs (SPC MapServer): ' + mesoscaleDiscussions.length + '\n\n';
                  
                  if (mesoscaleDiscussions.length > 0) {
                      info += '--- MCDs ---\n';
                      mesoscaleDiscussions.forEach((mcd, i) => {
                          const props  = mcd.properties || {};
                          const mcdNum = extractMCDNumber(props);
                          const parsed = parseMCDText(props._fullText || '');
                          info += (i+1) + '. MCD #' + mcdNum + '\n';
                          info += '   Geometry: ' + (mcd.geometry ? mcd.geometry.type : 'NONE') + '\n';
                          info += '   Issued: ' + (props.idp_filedate ? formatArcGISDate(props.idp_filedate) : 'unknown') + '\n';
                          info += '   Full text fetched: ' + (props._fullText ? 'yes (' + props._fullText.length + ' chars)' : 'no') + '\n';
                          if (parsed.concerning) info += '   Concerning: ' + parsed.concerning + '\n';
                          if (parsed.area)       info += '   Area: '       + parsed.area       + '\n';
                          info += '\n';
                      });
                  }
                  
                  warningsData.forEach((warning, i) => {
                      info += (i+1) + '. ' + (warning.type || 'Unknown') + ' - ' + (warning.area || 'Unknown') + '\n';
                      info += '   Severity: ' + (warning.severity || 'Unknown') + '\n';
                      info += '   Geometry: ' + (warning.geometry ? warning.geometry.type : 'NONE') + '\n';
                      if (warning.same && warning.same.length > 0) info += '   SAME: ' + warning.same.join(', ') + '\n';
                      info += '\n';
                  });
                  
                  debugDiv.textContent = info;
                  debugDiv.style.display = 'block';
              } else {
                  debugDiv.style.display = 'none';
              }
          }
          
          // Update all expiration countdowns
          function updateAllExpirationCountdowns() {
              document.querySelectorAll('[data-expires-timestamp]').forEach(function(element) {
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
                      let countdownText = '';
                      if (hours > 0) countdownText += hours + 'h ';
                      countdownText += minutes + 'm ' + seconds + 's';
                      element.textContent = countdownText;
                      element.classList.remove('urgent', 'warning');
                      if (timeLeft < 1800) element.classList.add('urgent');
                      else if (timeLeft < 7200) element.classList.add('warning');
                  }
              });
          }
       </script>
    </head>
    <body>
       <h1 style="text-align: center;">Active Weather Warnings</h1>
       
         {{ if .WarningTypeCounts }}
         <div class="warning-types" id="top">
            {{ range .WarningTypeCounts }}
               <div class="warning-type{{ if eq .Severity "Severe" }} severe{{ else if eq .Severity "Moderate" }} moderate{{ else if eq .Severity "Extreme" }} severe{{ else if eq .Severity "Minor" }}{{ else }} watch{{ end }}">
                 <a href="#{{ .Type | urlquery }}">{{ .Type }}</a>: {{ .Count }}
               </div>
            {{ end }}
         </div>
         {{ end }}
        
        <div class="warning-types">
           <div class="warning-type" style="background-color: #00aaaa;">
              <a href="#mcd-container">Mesoscale Discussions</a>: <span id="mcd-count">-</span>
           </div>
        </div>
       
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
          
           <!-- MCDs are injected here by JavaScript after NWS fetch -->
           <div id="mcd-container" style="margin-bottom: 20px;"></div>
          
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
             <div class="legend-color" style="background-color: #00aaaa; border-style: dashed;"></div>
             <span>Mesoscale Discussion (MCD) — NWS polygon</span>
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
             <strong>Note:</strong> Solid polygons show exact warning boundaries. Dashed outlines indicate county boundaries (fallback) or MCD areas.
          </div>
       </div>
       
       <a href="#top" class="back-to-top">↑ Top</a>
    </body>
    </html>
    `)
	if err != nil {
		return err
	}

	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return fmt.Errorf("failed to marshal warnings to JSON: %w", err)
	}

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

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return err
	}

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
	Severity string
}

func countWarningTypes(warnings []fetcher.Warning) (map[string]int, map[string]string) {
	typeCounts := make(map[string]int)
	severityMap := make(map[string]string)
	for _, warning := range warnings {
		if warning.Severity != "Header" {
			typeCounts[warning.Type]++
			currentSev := severityMap[warning.Type]
			if currentSev == "" || getSeverityRank(warning.Severity) > getSeverityRank(currentSev) {
				severityMap[warning.Type] = warning.Severity
			}
		}
	}
	return typeCounts, severityMap
}

func sortedWarningTypeCounts(warnings []fetcher.Warning) []TypeCount {
	typeCounts, severityMap := countWarningTypes(warnings)
	var result []TypeCount
	for warningType, count := range typeCounts {
		result = append(result, TypeCount{
			Type:     warningType,
			Count:    count,
			Priority: getWarningTypeRank(warningType),
			Severity: severityMap[warningType],
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].Count > result[j].Count
	})
	return result
}

// TemplateWarning is a wrapper for fetcher.Warning with additional template fields
type TemplateWarning struct {
	fetcher.Warning
	SeverityClass    string
	SeverityRank     int
	WarningTypeRank  int
	LocalIssued      string
	LocalExpires     string
	ExpiresTimestamp string
	ExtraClass       string
	ID               string
}

func getWarningTypeRank(warningType string) int {
	lowerType := strings.ToLower(warningType)
	if strings.Contains(lowerType, "tornado warning") {
		return 1
	}
	if strings.Contains(lowerType, "thunderstorm warning") ||
		strings.Contains(lowerType, "t-storm warning") ||
		strings.Contains(lowerType, "tstorm warning") {
		return 2
	}
	if strings.Contains(lowerType, "tornado") && strings.Contains(lowerType, "watch") {
		return 3
	}
	if (strings.Contains(lowerType, "thunderstorm") ||
		strings.Contains(lowerType, "t-storm") ||
		strings.Contains(lowerType, "tstorm")) &&
		strings.Contains(lowerType, "watch") {
		return 4
	}
	return 5
}

func convertWarnings(warnings []fetcher.Warning) []TemplateWarning {
	warningsByType := make(map[string][]TemplateWarning)
	var warningTypes []string

	for _, warning := range warnings {
		localIssued := formatToLocalTime(warning.Time)
		localExpires := formatToLocalTime(warning.ExpiresTime)
		expiresTimestamp := getExpiresTimestamp(warning.ExpiresTime)

		lowerType := strings.ToLower(warning.Type)
		isTornado := strings.Contains(lowerType, "tornado warning")
		isTornadoWatch := strings.Contains(lowerType, "tornado") && strings.Contains(lowerType, "watch")
		isThunderstormWatch := (strings.Contains(lowerType, "thunderstorm") ||
			strings.Contains(lowerType, "t-storm") ||
			strings.Contains(lowerType, "tstorm")) && strings.Contains(lowerType, "watch")
		isTstorm := strings.Contains(lowerType, "thunderstorm warning") ||
			strings.Contains(lowerType, "t-storm warning") ||
			strings.Contains(lowerType, "tstorm warning")

		var severityClass string
		switch {
		case isTornadoWatch:
			severityClass = "tornado-watch"
		case isThunderstormWatch:
			severityClass = "watch"
		case isTornado:
			severityClass = "tornado"
		case isTstorm:
			severityClass = "tstorm"
		default:
			severityClass = getSeverityClass(warning.Severity)
		}

		templateWarning := TemplateWarning{
			Warning:          warning,
			SeverityClass:    severityClass,
			SeverityRank:     getSeverityRank(warning.Severity),
			WarningTypeRank:  getWarningTypeRank(warning.Type),
			LocalIssued:      localIssued,
			LocalExpires:     localExpires,
			ExpiresTimestamp: expiresTimestamp,
			ExtraClass:       "",
			ID:               warning.ID,
		}

		if _, exists := warningsByType[warning.Type]; !exists {
			warningTypes = append(warningTypes, warning.Type)
		}
		warningsByType[warning.Type] = append(warningsByType[warning.Type], templateWarning)
	}

	for warningType := range warningsByType {
		sort.Slice(warningsByType[warningType], func(i, j int) bool {
			return warningsByType[warningType][i].SeverityRank > warningsByType[warningType][j].SeverityRank
		})
	}

	sort.Slice(warningTypes, func(i, j int) bool {
		iRank := getWarningTypeRank(warningTypes[i])
		jRank := getWarningTypeRank(warningTypes[j])
		if iRank != jRank {
			return iRank < jRank
		}
		iMax, jMax := 0, 0
		for _, w := range warningsByType[warningTypes[i]] {
			if w.SeverityRank > iMax {
				iMax = w.SeverityRank
			}
		}
		for _, w := range warningsByType[warningTypes[j]] {
			if w.SeverityRank > jMax {
				jMax = w.SeverityRank
			}
		}
		return iMax > jMax
	})

	var templateWarnings []TemplateWarning

	for _, warningType := range warningTypes {
		typeWarnings := warningsByType[warningType]
		lowerType := strings.ToLower(warningType)

		extraHeaderClass := ""
		switch {
		case strings.Contains(lowerType, "tornado warning"):
			extraHeaderClass = "tornado-header"
		case strings.Contains(lowerType, "tornado") && strings.Contains(lowerType, "watch"):
			extraHeaderClass = "tornado-watch-header"
		case (strings.Contains(lowerType, "thunderstorm") || strings.Contains(lowerType, "t-storm") || strings.Contains(lowerType, "tstorm")) && strings.Contains(lowerType, "watch"):
			extraHeaderClass = "watch-header"
		case strings.Contains(lowerType, "thunderstorm warning") || strings.Contains(lowerType, "t-storm warning") || strings.Contains(lowerType, "tstorm warning"):
			extraHeaderClass = "tstorm-header"
		}

		headerWarning := TemplateWarning{
			Warning: fetcher.Warning{
				Type:     warningType,
				Severity: "Header",
			},
			SeverityClass:   "header",
			SeverityRank:    0,
			WarningTypeRank: getWarningTypeRank(warningType),
			ExtraClass:      extraHeaderClass,
		}

		templateWarnings = append(templateWarnings, headerWarning)
		templateWarnings = append(templateWarnings, typeWarnings...)
	}

	return templateWarnings
}

func formatToLocalTime(timeStr string) string {
	if timeStr == "" {
		return "Not specified"
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return timeStr
		}
	}
	return t.Local().Format("Jan 2, 2006 at 3:04 PM MST")
}

func getExpiresTimestamp(timeStr string) string {
	if timeStr == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", timeStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, timeStr)
		if err != nil {
			return ""
		}
	}
	return fmt.Sprintf("%d", t.Unix())
}

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
	default:
		return 0
	}
}

// GenerateWarningsJSON creates a JSON file with current warnings data for AJAX updates
func GenerateWarningsJSON(warnings []fetcher.Warning, outputPath string) error {
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

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return os.WriteFile(outputPath, jsonData, 0644)
}
