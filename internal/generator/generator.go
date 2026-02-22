package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Zachdehooge/warnings-dashboard/internal/fetcher"
)

const nwsAlertsURL = "https://api.weather.gov/alerts/active?status=actual&event=Severe%20Thunderstorm%20Warning,Tornado%20Warning,Tornado%20Watch,Severe%20Thunderstorm%20Watch,Special%20Weather%20Statement"

// WarningJSON is the shape written to warnings.json and consumed by the browser JS.
type WarningJSON struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"`
	Description string       `json:"description"`
	Area        string       `json:"area"`
	Severity    string       `json:"severity"`
	Time        string       `json:"time"`
	ExpiresTime string       `json:"expiresTime"`
	Geometry    *GeoGeometry `json:"geometry"`
	UGC         []string     `json:"ugc"`
	SAME        []string     `json:"same"`
}

// GeoGeometry mirrors the GeoJSON geometry object the frontend expects.
type GeoGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// MesoscaleDiscussionJSON represents an MCD from the NOAA MapServer
type MesoscaleDiscussionJSON struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	FullText  string       `json:"fullText"`
	PopupInfo string       `json:"popupInfo"`
	FileDate  int64        `json:"idp_filedate"`
	Geometry  *GeoGeometry `json:"geometry"`
}

// PolledPayload is the full structure written to warnings.json on every poll cycle.
type PolledPayload struct {
	Warnings             []WarningJSON             `json:"warnings"`
	MesoscaleDiscussions []MesoscaleDiscussionJSON `json:"mesoscaleDiscussions"`
	LastUpdated          string                    `json:"lastUpdated"`
	Counter              int                       `json:"counter"`
	UpdatedAtUTC         int64                     `json:"updatedAtUTC"`
}

// StartPoller launches a background goroutine that polls the NWS API every
// interval and atomically rewrites outputPath (e.g. "warnings.json").
// Call once from main() after generating the initial HTML.
func StartPoller(outputPath string, interval time.Duration) {
	if err := pollAndWrite(outputPath); err != nil {
		log.Printf("[poller] initial poll error: %v", err)
	}
	go func() {
		for {
			time.Sleep(interval)
			if err := pollAndWrite(outputPath); err != nil {
				log.Printf("[poller] poll error: %v", err)
			}
		}
	}()
	log.Printf("[poller] started — writing to %s every %s", outputPath, interval)
}

// pollAndWrite fetches all pages from NWS and atomically writes warnings.json.
func pollAndWrite(outputPath string) error {
	warnings, err := fetchAllAlerts()
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	mCDs, err := fetchMesoscaleDiscussions()
	if err != nil {
		log.Printf("[poller] MCD fetch failed: %v", err)
		mCDs = []MesoscaleDiscussionJSON{}
	}
	log.Printf("DEBUG: mCDs slice len=%d, cap=%d", len(mCDs), cap(mCDs))

	payload := PolledPayload{
		Warnings:             warnings,
		MesoscaleDiscussions: mCDs,
		LastUpdated:          time.Now().UTC().Format("Jan 2, 2006 at 03:04:01 UTC"),
		Counter:              len(warnings),
		UpdatedAtUTC:         time.Now().UTC().Unix(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// Write to a temp file then rename — prevents the browser reading a partial file.
	tmp := outputPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp failed: %w", err)
	}
	if err := os.Rename(tmp, outputPath); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	log.Printf("[poller] %d active warnings written to %s", len(warnings), outputPath)
	return nil
}

// fetchAllAlerts pages through the NWS API and returns all active alerts.
func fetchAllAlerts() ([]WarningJSON, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var all []WarningJSON
	url := nwsAlertsURL

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "warnings-dashboard/1.0 (github.com/Zachdehooge/warnings-dashboard)")
		req.Header.Set("Accept", "application/geo+json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP GET failed: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read body failed: %w", err)
		}
		if resp.StatusCode != 200 {
			snip := body
			if len(snip) > 200 {
				snip = snip[:200]
			}
			return nil, fmt.Errorf("NWS returned HTTP %d: %s", resp.StatusCode, string(snip))
		}

		var apiResp struct {
			Features []struct {
				Geometry *struct {
					Type        string          `json:"type"`
					Coordinates json.RawMessage `json:"coordinates"`
				} `json:"geometry"`
				Properties struct {
					ID          string `json:"id"`
					Event       string `json:"event"`
					Description string `json:"description"`
					AreaDesc    string `json:"areaDesc"`
					Severity    string `json:"severity"`
					Status      string `json:"status"`
					Sent        string `json:"sent"`
					Expires     string `json:"expires"`
					Geocode     struct {
						UGC  []string `json:"UGC"`
						SAME []string `json:"SAME"`
					} `json:"geocode"`
				} `json:"properties"`
			} `json:"features"`
			Pagination *struct {
				Next string `json:"next"`
			} `json:"pagination"`
		}

		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("JSON decode failed: %w", err)
		}

		now := time.Now()
		for _, f := range apiResp.Features {
			p := f.Properties

			if p.Status != "Actual" {
				continue
			}
			// Skip already-expired alerts
			if p.Expires != "" {
				if exp, err := time.Parse(time.RFC3339, p.Expires); err == nil && exp.Before(now) {
					continue
				}
			}

			ugc := p.Geocode.UGC
			if ugc == nil {
				ugc = []string{}
			}
			same := p.Geocode.SAME
			if same == nil {
				same = []string{}
			}

			w := WarningJSON{
				ID:          p.ID,
				Type:        p.Event,
				Description: p.Description,
				Area:        p.AreaDesc,
				Severity:    p.Severity,
				Time:        p.Sent,
				ExpiresTime: p.Expires,
				UGC:         ugc,
				SAME:        same,
			}
			if f.Geometry != nil {
				w.Geometry = &GeoGeometry{
					Type:        f.Geometry.Type,
					Coordinates: f.Geometry.Coordinates,
				}
			}
			all = append(all, w)
		}

		if apiResp.Pagination != nil && apiResp.Pagination.Next != "" {
			url = apiResp.Pagination.Next
		} else {
			url = ""
		}
	}

	if all == nil {
		all = []WarningJSON{}
	}
	return all, nil
}

func fetchMesoscaleDiscussions() ([]MesoscaleDiscussionJSON, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	mcdURL := "https://mapservices.weather.noaa.gov/vector/rest/services/outlooks/spc_mesoscale_discussion/MapServer/0/query"
	req, err := http.NewRequest("GET", mcdURL+"?where=1=1&outFields=name,folderpath,popupinfo,idp_filedate&f=geojson", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MCD MapServer fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MCD MapServer returned HTTP %d", resp.StatusCode)
	}

	var geoResp struct {
		Features []struct {
			Geometry   *GeoGeometry `json:"geometry"`
			Properties struct {
				Name       string `json:"name"`
				FolderPath string `json:"folderpath"`
				PopupInfo  string `json:"popupinfo"`
				FileDate   int64  `json:"idp_filedate"`
			} `json:"properties"`
		} `json:"features"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCD response: %w", err)
	}

	if err := json.Unmarshal(body, &geoResp); err != nil {
		return nil, fmt.Errorf("failed to parse MCD JSON: %w", err)
	}

	var mCDs []MesoscaleDiscussionJSON
	year := time.Now().Year()

	for _, f := range geoResp.Features {
		if f.Geometry == nil {
			continue
		}

		props := f.Properties
		mcdNum := strings.TrimPrefix(props.Name, "MD ")
		if mcdNum == "" {
			continue
		}

		fullText, err := fetchMCDText(year, mcdNum)
		if err != nil {
			log.Printf("[mcd] failed to fetch text for MCD %s: %v", mcdNum, err)
		}

		mCDs = append(mCDs, MesoscaleDiscussionJSON{
			ID:        "MCD " + mcdNum,
			Name:      props.Name,
			FullText:  fullText,
			PopupInfo: props.PopupInfo,
			FileDate:  props.FileDate,
			Geometry:  f.Geometry,
		})
	}

	if mCDs == nil {
		mCDs = []MesoscaleDiscussionJSON{}
	}
	return mCDs, nil
}

func fetchMCDText(year int, mcdNum string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://www.spc.noaa.gov/products/md/%d/md%s.html", year, mcdNum)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "warnings-dashboard/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`(?s)<pre[^>]*>(.*?)</pre>`)
	matches := re.FindSubmatch(body)
	if len(matches) < 2 {
		return "", nil
	}

	text := string(matches[1])
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	reTag := regexp.MustCompile(`<[^>]+>`)
	text = reTag.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return text, nil
}

// GenerateWarningsHTML creates an HTML file with weather warnings
func GenerateWarningsHTML(warnings []fetcher.Warning, outputPath string) error {
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
          @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
          html { background-color: #121212; }
          .warning-title:hover { text-decoration: underline; color: #add8e6; }
          #map {
             height: 600px; width: 100%;
             border: 2px solid var(--card-border);
             border-radius: 5px; margin-top: 20px;
             opacity: 1; transition: opacity 0.2s ease-in-out;
          }
          #map.loading { opacity: 0.7; }
          .map-legend {
             background-color: var(--card-bg); padding: 10px;
             border-radius: 5px; margin-top: 10px;
             border: 1px solid var(--card-border);
          }
          .legend-item { display: flex; align-items: center; margin: 5px 0; }
          .legend-color { width: 30px; height: 20px; margin-right: 10px; border: 1px solid #fff; }
          .warnings-container {
             display: grid;
             grid-template-columns: repeat(auto-fit, minmax(450px, 1fr));
             gap: 15px;
          }
          .warning {
             border: 1px solid var(--card-border); padding: 10px;
             border-radius: 5px; background-color: var(--card-bg); height: 100%;
          }
          .warning.severe   { background-color: var(--severe-bg);        border-color: var(--severe-border); }
          .warning.moderate { background-color: var(--moderate-bg);       border-color: var(--moderate-border); }
          .warning.tornado  { background-color: var(--tornado-bg);        border-color: var(--tornado-border); }
          .warning.watch    { background-color: var(--watch-bg);          border-color: var(--watch-border); }
          .warning.tornado-watch { background-color: var(--tornado-watch-bg); border-color: var(--tornado-watch-border); }
          .warning.tstorm   { background-color: var(--tstorm-bg);         border-color: var(--tstorm-border); }
          .warning.mcd      { background-color: var(--mcd-bg);            border-color: var(--mcd-border); }
          .warning.header {
             background-color: var(--header-bg); border-color: var(--header-border);
             grid-column: 1 / -1; margin-top: 15px; margin-bottom: 5px;
             padding: 2px 10px; border-width: 2px; height: auto;
          }
          .warning.header.tornado-header      { background-color: var(--tornado-bg);       border-color: var(--tornado-border); }
          .warning.header.tornado-watch-header{ background-color: var(--tornado-watch-bg); border-color: var(--tornado-watch-border); }
          .warning.header.watch-header        { background-color: var(--watch-bg);         border-color: var(--watch-border); }
          .warning.header.tstorm-header       { background-color: var(--tstorm-bg);        border-color: var(--tstorm-border); }
          .warning.header.mcd-header          { background-color: var(--mcd-bg);           border-color: var(--mcd-border); }
          .warning.header h2 { margin: 5px 0; text-align: center; font-size: 1.2em; }
          .warning-header { display: flex; justify-content: space-between; align-items: center; }
          .warning-types {
             margin-bottom: 15px; background-color: var(--summary-bg);
             padding: 15px; border-radius: 5px; display: flex; flex-wrap: wrap; align-items: center;
          }
          .warning-types h2 { margin-right: 15px; margin-bottom: 5px; }
          .warning-type {
             margin: 5px 0; padding: 3px 8px; border-radius: 3px;
             display: inline-block; margin-right: 10px;
          }
          .warning-type.tornado      { background-color: var(--tornado-bg);       border: 1px solid var(--tornado-border); }
          .warning-type.tstorm       { background-color: var(--tstorm-bg);        border: 1px solid var(--tstorm-border); }
          .warning-type.tornado-watch{ background-color: var(--tornado-watch-bg); border: 1px solid var(--tornado-watch-border); }
          .warning-type.watch        { background-color: var(--watch-bg);         border: 1px solid var(--watch-border); }
          .warning-type.mcd          { background-color: var(--mcd-bg);           border: 1px solid var(--mcd-border); }
          .warning-type.moderate     { background-color: #4d3510;                 border: 1px solid #b25900; }
          .warning-type.severe       { background-color: var(--severe-bg);        border: 1px solid var(--severe-border); }
          .warning-type a { color: var(--text-color); text-decoration: none; transition: color 0.2s; }
          .warning-type a:hover { color: #add8e6; text-decoration: underline; }
          .back-to-top {
             position: fixed; bottom: 20px; right: 20px;
             background-color: var(--header-bg); color: var(--text-color);
             padding: 10px 15px; border-radius: 5px; border: 1px solid var(--header-border);
             cursor: pointer; text-decoration: none; opacity: 0.8; transition: opacity 0.2s;
          }
          .back-to-top:hover { opacity: 1; }
          .leaflet-control-reset-map {
             background-color: var(--header-bg); color: var(--text-color);
             padding: 8px 12px; border-radius: 4px; border: 1px solid var(--header-border);
             cursor: pointer; font-size: 14px; font-family: Arial, sans-serif;
          }
          .leaflet-control-reset-map:hover { background-color: var(--tab-active-bg); }
          h1, h2, h4 { color: var(--text-color); }
          .next-refresh { font-size: 0.8em; margin-top: 10px; color: #888; }
          .expiration-time { display: flex; justify-content: space-between; align-items: center; margin-top: 5px; }
          .expiration-countdown { font-weight: bold; }
          .expiration-countdown.urgent  { color: var(--countdown-warning); }
          .expiration-countdown.warning { color: var(--countdown-caution); }
          .mcd-description {
             font-size: 0.85em; color: #aaa; margin-top: 8px; max-height: 80px;
             overflow-y: auto; border-top: 1px solid var(--mcd-border); padding-top: 6px;
          }
          @media (max-width: 768px) { .warnings-container { grid-template-columns: 1fr; } }
       </style>
       <script>
          let map;
          let warningsData = {{ .WarningsJSON }};
          let countyBoundaries = null;
           let warningLayers = [];
           let mesoscaleDiscussions = [];
           let validMCDs = [];
           let lastUpdateTime = Date.now();

           // ------------------------------------------------------------------
          // POLL: read warnings.json written by the Go poller every 15 seconds.
          // Fast (local file), no CORS issues, no NWS rate limits.
          // ------------------------------------------------------------------
          async function fetchUpdatedWarnings() {
              try {
                  const response = await fetch('warnings.json?_=' + Date.now());
                  if (!response.ok) throw new Error('Failed to read warnings.json: ' + response.status);

                  const payload = await response.json();
                  console.log('[poll] ' + (payload.warnings || []).length + ' warnings from warnings.json, updated ' + payload.lastUpdated);

                  // Server already filtered expired alerts, but double-check client-side
                  const now = Date.now();
                  warningsData = (payload.warnings || []).filter(w =>
                      !w.expiresTime || new Date(w.expiresTime).getTime() > now
                  );

                  // Read MCDs from the server-written JSON
                  mesoscaleDiscussions = (payload.mesoscaleDiscussions || []).map(mcd => ({
                      type: 'Feature',
                      geometry: mcd.geometry,
                      properties: {
                          name: mcd.name,
                          folderpath: '',
                          popupinfo: mcd.popupInfo,
                          idp_filedate: mcd.idp_filedate,
                          _fullText: mcd.fullText || ''
                      }
                  }));
                  console.log('[poll] MCDs from server: ' + mesoscaleDiscussions.length);

                  lastUpdateTime = Date.now();
                  updateStats({ counter: warningsData.length, updatedAtUTC: payload.updatedAtUTC });

                  clearWarningLayers();
                  addMesoscaleDiscussionsToMap();
                  addWarningsToMap();
                  bringSevereToFront();
                  if (radarLayer && map.hasLayer(radarLayer)) {
                      radarLayer.bringToBack();
                  }
                  addMesoscaleDiscussionsToList();
                  updateListView(warningsData);
                  console.log('[poll] map and list updated with ' + warningsData.length + ' warnings');

              } catch (error) {
                  console.error('[poll] error reading warnings.json:', error);
              }
          }

          // ------------------------------------------------------------------
          // Strip Z coordinate from GeoJSON geometry
          // ------------------------------------------------------------------
          function stripZ(geometry) {
              if (!geometry) return geometry;
              const strip2D = coords => coords.map(c => [c[0], c[1]]);
              switch (geometry.type) {
                  case 'Polygon':     return { type: 'Polygon',     coordinates: geometry.coordinates.map(strip2D) };
                  case 'MultiPolygon':return { type: 'MultiPolygon', coordinates: geometry.coordinates.map(ring => ring.map(strip2D)) };
                  default: return geometry;
              }
          }

          function extractMCDNumber(props) {
              if (!props.name) return '????';
              const digits = String(props.name).replace(/[^0-9]/g, '');
              return digits ? digits : '????';
          }

          function formatArcGISDate(msTimestamp) {
              if (!msTimestamp && msTimestamp !== 0) return 'Not specified';
              return new Date(msTimestamp).toLocaleString(undefined, {
                  year:'numeric', month:'short', day:'numeric',
                  hour:'numeric', minute:'2-digit', second:'2-digit', timeZoneName:'short'
              });
          }

          function extractExpireTime(validStr) {
              if (!validStr) return '';
              const match = validStr.match(/-\s*(\d{6}Z)/);
              return match ? match[1] : '';
          }

          function formatExpireToLocal(utcStr, rawText) {
              if (!utcStr || utcStr.length !== 7) return utcStr;
              const day = parseInt(utcStr.substring(0,2));
              const hour = parseInt(utcStr.substring(2,4));
              const min = parseInt(utcStr.substring(4,6));
              let month = new Date().getUTCMonth(), year = new Date().getUTCFullYear();
              if (rawText) {
                  const mm = rawText.match(/\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{4}/i);
                  if (mm) {
                      const months = {'Jan':0,'Feb':1,'Mar':2,'Apr':3,'May':4,'Jun':5,'Jul':6,'Aug':7,'Sep':8,'Oct':9,'Nov':10,'Dec':11};
                      month = months[mm[1]];
                      const ym = rawText.match(/[A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+(\d{4})/);
                      if (ym) year = parseInt(ym[1]);
                  }
              }
              return new Date(Date.UTC(year, month, day, hour, min)).toLocaleString(undefined, {
                  year:'numeric', month:'short', day:'numeric', hour:'numeric', minute:'2-digit', timeZoneName:'short'
              });
          }

          function mcdSPCLink(mcdNum) {
              return 'https://www.spc.noaa.gov/products/md/' + new Date().getFullYear() + '/md' + mcdNum + '.html';
          }

          function parseMCDText(rawText) {
              if (!rawText) return { area:'', concerning:'', valid:'', summary:'', discussion:'', probability:'', raw:'' };
              const startIdx = rawText.indexOf('MESOSCALE DISCUSSION');
              const text = startIdx >= 0 ? rawText.substring(startIdx) : rawText;
              const getInline = (label) => {
                  const m = text.match(new RegExp(label + '\\.{3}(.+)', 'i'));
                  if (m) return m[1].trim();
                  const m2 = text.match(new RegExp(label + '\\s+(.+)', 'i'));
                  return m2 ? m2[1].trim() : '';
              };
              const getBlock = (label) => {
                  const m = text.match(new RegExp(label + '\\.{3}[\\s\\S]*?(?=\\n[A-Z][A-Z .]{2,}\\.{3}|\\n\\.\\.|[A-Z][A-Z]+\\.{3}|\\nMESOSCALE|$)', 'i'));
                  if (!m) return '';
                  return m[0].replace(new RegExp('^' + label + '\\.{3}', 'i'), '').replace(/\s+/g, ' ').trim();
              };
              return {
                  area: getInline('AREA AFFECTED'), concerning: getInline('CONCERNING'),
                  valid: getInline('VALID'), summary: getBlock('SUMMARY'),
                  discussion: getBlock('DISCUSSION'), probability: getInline('PROBABILITY OF WATCH ISSUANCE'),
                  raw: text
              };
          }

          function parseIssuedTime(rawText) {
              if (!rawText) return null;
              const match = rawText.match(/(\d{2})(\d{2})\s+(AM|PM)\s+(CST|CDT|MST|MDT|EST|EDT|PST|PDT)\s+([A-Z][a-z]{2})\s+([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{4})/i);
              if (!match) return null;
              let hour = parseInt(match[1]);
              const minute = parseInt(match[2]);
              const ampm = match[3].toUpperCase();
              if (ampm === 'PM' && hour !== 12) hour += 12;
              if (ampm === 'AM' && hour === 12) hour = 0;
              const months = {'Jan':0,'Feb':1,'Mar':2,'Apr':3,'May':4,'Jun':5,'Jul':6,'Aug':7,'Sep':8,'Oct':9,'Nov':10,'Dec':11};
              const tzOffsets = {'CST':6,'CDT':5,'MST':7,'MDT':6,'EST':5,'EDT':4,'PST':8,'PDT':7};
              const offset = tzOffsets[match[4].toUpperCase()] || 0;
              const utcHour = (hour + offset) % 24;
              const utcDay = utcHour < hour ? parseInt(match[7]) + 1 : parseInt(match[7]);
              const date = new Date(Date.UTC(parseInt(match[8]), months[match[6]], utcDay, utcHour, minute, 0));
              return isNaN(date.getTime()) ? null : date;
          }

          function formatDateToLocal(dateObj) {
              if (!dateObj || isNaN(dateObj.getTime())) return 'Not specified';
              return dateObj.toLocaleString(undefined, {
                  year:'numeric', month:'short', day:'numeric',
                  hour:'numeric', minute:'2-digit', second:'2-digit', timeZoneName:'short'
              });
          }

          async function loadCountyBoundaries() {
              try {
                  const response = await fetch('https://raw.githubusercontent.com/plotly/datasets/master/geojson-counties-fips.json');
                  countyBoundaries = await response.json();
                  console.log('County boundaries loaded successfully');
              } catch (error) {
                  console.error('Failed to load county boundaries:', error);
              }
          }

          function clearWarningLayers() {
              warningLayers.forEach(layer => { if (map.hasLayer(layer)) map.removeLayer(layer); });
              warningLayers = [];
          }

          function updateStats(data) {
              console.log('updateStats called with counter:', data.counter);
              if (data.updatedAtUTC) updateLastUpdatedTime(data.updatedAtUTC);
              document.querySelectorAll('h4').forEach(el => {
                  if (el.textContent.startsWith('Warnings:')) el.textContent = 'Warnings: ' + data.counter;
              });
              updateWarningTypeCounts();
          }

          function updateWarningTypeCounts() {
              const typeCounts = {};
              warningsData.forEach(w => {
                  if (!w.type) return;
                  if (!typeCounts[w.type]) typeCounts[w.type] = 0;
                  typeCounts[w.type]++;
              });
              const warningTypesDiv = document.getElementById('top');
              if (!warningTypesDiv) return;
              const types = Object.keys(typeCounts).sort();
              if (types.length === 0) {
                  warningTypesDiv.innerHTML = '';
                  return;
              }
              let html = '';
              types.forEach(type => {
                  const count = typeCounts[type];
                  const severityClass = getWarningSeverityClass({ type: type, severity: '' });
                  html += '<div class="warning-type ' + severityClass + '">' +
                      '<a href="#' + encodeURIComponent(type) + '">' + type + '</a>: <span class="type-count">' + count + '</span></div>';
              });
              warningTypesDiv.innerHTML = html;
          }

          // ------------------------------------------------------------------
          // MCD list rendering
          // ------------------------------------------------------------------
          function addMesoscaleDiscussionsToList() {
              const container = document.getElementById('mcd-container');
              if (!container) return;

              validMCDs = mesoscaleDiscussions.filter(mcd => extractMCDNumber(mcd.properties || {}) !== '????');

              const mcdCountEl = document.getElementById('mcd-count');
              if (mcdCountEl) mcdCountEl.textContent = validMCDs.length;
              const mcdTotalEl = document.getElementById('mcd-total-count');
              if (mcdTotalEl) mcdTotalEl.textContent = validMCDs.length;

              if (validMCDs.length === 0) { container.innerHTML = ''; return; }

              let html = '<div class="warning header mcd-header" style="grid-column:1/-1;margin-top:20px;">' +
                  '<h2>Mesoscale Discussions (' + validMCDs.length + ')</h2></div>' +
                  '<div class="warnings-container">';

              validMCDs.forEach((mcd, index) => {
                  const props = mcd.properties || {};
                  const mcdNum = extractMCDNumber(props);
                  const parsed = parseMCDText(props._fullText || '');
                  const issuedDate = parseIssuedTime(props._fullText);
                  const issued = issuedDate ? formatDateToLocal(issuedDate) : (props.idp_filedate ? formatArcGISDate(props.idp_filedate) : 'Not specified');
                  const spcUrl = mcdSPCLink(mcdNum);
                  const expire = extractExpireTime(parsed.valid);

                  html += '<div class="warning mcd">';
                  html += '<div class="warning-header"><h2 class="warning-title" style="cursor:pointer;" onclick="zoomToMCD(' + index + ')">MCD #' + mcdNum + '</h2>';
                  if (parsed.probability) html += '<span style="color:#ff6600;font-weight:bold;">POWI: ' + escapeHtml(parsed.probability) + '</span>';
                  html += '</div>';
                  html += '<small><strong>Issued:</strong> ' + escapeHtml(issued) + '</small>';
                  if (expire) html += '<small style="display:block;"><strong>Expires:</strong> ' + escapeHtml(formatExpireToLocal(expire, props._fullText)) + '</small>';
                  if (parsed.area) html += '<p style="margin:6px 0 2px;"><strong>Area:</strong> ' + escapeHtml(parsed.area) + '</p>';
                  if (parsed.concerning) html += '<p style="margin:2px 0;"><strong>Concerning:</strong> ' + escapeHtml(parsed.concerning) + '</p>';
                  if (parsed.summary) html += '<div class="mcd-description" style="max-height:120px;"><strong>Summary:</strong> ' + escapeHtml(parsed.summary) + '</div>';
                  if (parsed.discussion) html += '<div class="mcd-description" style="max-height:150px;margin-top:6px;"><strong>Discussion:</strong> ' + escapeHtml(parsed.discussion) + '</div>';
                  else if (!parsed.summary && props._fullText) {
                      const pt = props._fullText.replace(/\s+/g,' ').trim();
                      if (pt) html += '<div class="mcd-description">' + escapeHtml(pt.substring(0,500)) + (pt.length>500?'…':'') + '</div>';
                  }
                  html += '<div style="margin-top:8px;"><small><a href="' + spcUrl + '" target="_blank" style="color:#00aaaa;">View full discussion on SPC ↗</a></small></div>';
                  html += '</div>';
              });
              html += '</div>';
              container.innerHTML = html;
              updateAllExpirationCountdowns();
          }

          function escapeHtml(str) {
              return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
          }

          function zoomToMCD(index) {
              document.getElementById('map').scrollIntoView({ behavior:'smooth', block:'center' });
              const mcd = validMCDs[index];
              if (!mcd || !mcd.geometry) return;
              try {
                  let bounds;
                  if (mcd.geometry.type === 'Polygon') {
                      bounds = L.latLngBounds(mcd.geometry.coordinates[0].map(c => [c[1],c[0]]));
                  } else if (mcd.geometry.type === 'MultiPolygon') {
                      bounds = L.latLngBounds(mcd.geometry.coordinates.flat(1).map(c => [c[1],c[0]]));
                  }
                  if (bounds && bounds.isValid()) map.fitBounds(bounds, { padding:[50,50] });
              } catch (e) { console.error('Error zooming to MCD:', e); }
          }

          function getSeverityClassJS(severity) {
              if (!severity) return 'other';
              const s = severity.toLowerCase();
              if (s==='extreme'||s==='severe') return 'severe';
              if (s==='moderate') return 'moderate';
              return 'other';
          }

          function getWarningSeverityClass(warning) {
              const t = (warning.type || '').toLowerCase();
              if (t.includes('tornado warning')) return 'tornado';
              if (t.includes('tornado') && t.includes('watch')) return 'tornado-watch';
              if ((t.includes('thunderstorm')||t.includes('t-storm')||t.includes('tstorm')) && t.includes('watch')) return 'watch';
              if (t.includes('thunderstorm warning')||t.includes('t-storm warning')||t.includes('tstorm warning')) return 'tstorm';
              return getSeverityClassJS(warning.severity);
          }

          function parseISOTime(isoString) {
              if (!isoString) return '';
              try {
                  return new Date(isoString).toLocaleString(undefined, {
                      year:'numeric', month:'short', day:'numeric',
                      hour:'numeric', minute:'2-digit', second:'2-digit', timeZoneName:'short'
                  });
              } catch (e) { return isoString; }
          }

          function getExpiresTimestampJS(isoString) {
              if (!isoString) return '';
              try { return Math.floor(new Date(isoString).getTime() / 1000); } catch(e) { return ''; }
          }

          function renderWarningCard(warning) {
              const severityClass = getWarningSeverityClass(warning);
              const expiresTimestamp = getExpiresTimestampJS(warning.expiresTime);
              return '<div class="warning ' + severityClass + '" data-warning-id="' + warning.id + '">' +
                  '<div class="warning-header">' +
                      '<h2 class="warning-title" style="cursor:pointer;" onclick="zoomToWarning(\'' + warning.id + '\')">' + (warning.type||'') + '</h2>' +
                      '<strong>' + (warning.severity||'') + ' Severity</strong>' +
                  '</div>' +
                  '<p><strong>Area:</strong> ' + (warning.area||'') + '</p>' +
                  '<p>' + (warning.description||'') + '</p>' +
                  '<small>Issued: ' + parseISOTime(warning.time) + '</small><br>' +
                  '<div class="expiration-time">' +
                      '<small>Expires: ' + parseISOTime(warning.expiresTime) + '</small>' +
                      '<small class="expiration-countdown" data-expires-timestamp="' + expiresTimestamp + '"></small>' +
                  '</div>' +
              '</div>';
          }

          function renderWarningsList(warnings) {
              const byType = {};
              warnings.forEach(w => { if (!byType[w.type]) byType[w.type]=[]; byType[w.type].push(w); });
              const typeOrder = ['Tornado Warning','Severe Thunderstorm Warning','Tornado Watch','Severe Thunderstorm Watch'];
              const sortedTypes = Object.keys(byType).sort((a,b) => {
                  const ia = typeOrder.indexOf(a), ib = typeOrder.indexOf(b);
                  if (ia!==-1&&ib!==-1) return ia-ib;
                  if (ia!==-1) return -1; if (ib!==-1) return 1;
                  return a.localeCompare(b);
              });
              let html = '<div class="warnings-container">';
              sortedTypes.forEach(type => {
                  const sc = getWarningSeverityClass(byType[type][0]);
                  html += '<div class="warning header ' + sc + '" id="' + encodeURIComponent(type) + '"><h2>' + type + '</h2></div>';
                  byType[type].forEach(w => { html += renderWarningCard(w); });
              });
              return html + '</div>';
          }

          function updateListView(warnings) {
              const listSection = document.getElementById('list-section');
              if (!listSection) return;
              const oldMcd = document.getElementById('mcd-container');
              const savedMcdHTML = oldMcd ? oldMcd.innerHTML : '';
              if (warnings.length === 0) {
                  listSection.innerHTML = '<div id="mcd-container" style="margin-bottom:20px;">' + savedMcdHTML + '</div><p>No active weather warnings at this time.</p>';
              } else {
                  listSection.innerHTML = '<div id="mcd-container" style="margin-bottom:20px;">' + savedMcdHTML + '</div>' + renderWarningsList(warnings);
              }
              updateAllExpirationCountdowns();
          }

          function formatLocalTime(timestamp) {
              return new Date(timestamp * 1000).toLocaleString(undefined, {
                  year:'numeric', month:'short', day:'numeric',
                  hour:'numeric', minute:'2-digit', second:'2-digit', timeZoneName:'short'
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

              // Countdown display - synchronized with fetch
              const refreshInterval = 15000;
              let refreshTime = 15;
              const countdownElements = document.querySelectorAll('.countdown');
              
              function updateCountdown() {
                  countdownElements.forEach(el => el.textContent = refreshTime + 's');
              }
              
              setInterval(function() {
                  refreshTime--;
                  if (refreshTime <= 0) {
                      countdownElements.forEach(el => el.textContent = 'Updating...');
                  } else {
                      updateCountdown();
                  }
              }, 1000);
              updateCountdown();

              // Poll warnings.json every 15 seconds
              setInterval(function() {
                  refreshTime = 15;
                  updateCountdown();
                  fetchUpdatedWarnings();
              }, refreshInterval);
              fetchUpdatedWarnings(); // Fetch immediately on page load

              const backToTopButton = document.querySelector('.back-to-top');
              window.addEventListener('scroll', function() {
                  backToTopButton.style.display = window.scrollY > 300 ? 'block' : 'none';
              });
              if (window.scrollY <= 300) backToTopButton.style.display = 'none';

              updateAllExpirationCountdowns();

              // Every second: prune client-side expired warnings and tick countdowns
              setInterval(function() {
                  const now = Date.now();
                  const before = warningsData.length;
                  warningsData = warningsData.filter(w => !w.expiresTime || new Date(w.expiresTime).getTime() > now);
                  if (warningsData.length !== before) {
                      console.log('[prune] removed ' + (before - warningsData.length) + ' expired warning(s)');
                      clearWarningLayers();
                      addMesoscaleDiscussionsToMap();
                      addWarningsToMap();
                      bringSevereToFront();
                      updateListView(warningsData);
                      document.querySelectorAll('h4').forEach(el => {
                          if (el.textContent.startsWith('Warnings:')) el.textContent = 'Warnings: ' + warningsData.length;
                      });
                      updateWarningTypeCounts();
                  }
                  updateAllExpirationCountdowns();
              }, 1000);
          };

          function zoomToWarning(warningId) {
              document.getElementById('map').scrollIntoView({ behavior:'smooth', block:'center' });
              const warning = warningsData.find(w => w.id === warningId);
              if (!warning) return;
              const layer = warningLayers.find(l => l._popup && l._popup._content &&
                  l._popup._content.includes(warning.type) && l._popup._content.includes(warning.area));
              if (layer && layer.getBounds) {
                  map.fitBounds(layer.getBounds(), { padding:[50,50] });
                  setTimeout(() => layer.openPopup(), 500);
              } else if (warning.geometry && warning.geometry.coordinates) {
                  try {
                      const coords = warning.geometry.type === 'Polygon'
                          ? warning.geometry.coordinates[0]
                          : warning.geometry.coordinates[0][0];
                      map.fitBounds(L.latLngBounds(coords.map(c => [c[1],c[0]])), { padding:[50,50] });
                  } catch(e) { console.error('Error zooming to warning:', e); }
              }
          }

          async function initMap() {
              map = L.map('map').setView([39.8283, -98.5795], 4);

              const savedMapState = localStorage.getItem('mapState');
              if (savedMapState) {
                  try { const s = JSON.parse(savedMapState); map.setView([s.lat, s.lng], s.zoom); } catch(e) {}
              }

              L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
                  attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
                  subdomains: 'abcd', maxZoom: 20
              }).addTo(map);

              // Add radar reflectivity layer (Iowa Environmental Mesonet)
              const radarLayer = L.tileLayer.wms('https://mesonet.agron.iastate.edu/cgi-bin/wms/nexrad/n0q.cgi', {
                  layers: 'nexrad-n0q-m05m',
                  format: 'image/png',
                  transparent: true,
                  opacity: 0.5,
                  maxZoom: 12,
                  attribution: 'Radar data &copy; Iowa Environmental Mesonet'
              });

               // Layer control for toggling radar only
               const overlays = {
                   "Radar": radarLayer
               };
               L.control.layers(null, overlays, { collapsed: false, autoZIndex: false }).addTo(map);

              const initialView = [39.8283, -98.5795], initialZoom = 4;
              L.Control.ResetMap = L.Control.extend({
                  onAdd: function(map) {
                      const btn = L.DomUtil.create('button', 'leaflet-control-reset-map');
                      btn.innerHTML = '⟲ Reset Map'; btn.title = 'Reset to full US view';
                      btn.onclick = function(e) {
                          L.DomEvent.stopPropagation(e);
                          map.setView(initialView, initialZoom);
                          localStorage.removeItem('mapState');
                      };
                      return btn;
                  }
              });
              L.control.resetMap = opts => new L.Control.ResetMap(opts);
              L.control.resetMap({ position: 'topright' }).addTo(map);
              map.on('moveend', saveMapState);
              map.on('zoomend', saveMapState);

              // Render initial data immediately
              addWarningsToMap();
              updateListView(warningsData);

              loadCountyBoundaries();
          }

          function saveMapState() {
              const c = map.getCenter();
              localStorage.setItem('mapState', JSON.stringify({ lat: c.lat, lng: c.lng, zoom: map.getZoom() }));
          }

          function addMesoscaleDiscussionsToMap() {
              if (!mesoscaleDiscussions || mesoscaleDiscussions.length === 0) return;
              validMCDs = mesoscaleDiscussions.filter(mcd => extractMCDNumber(mcd.properties || {}) !== '????');
              if (validMCDs.length === 0) return;

              validMCDs.forEach((mcd, index) => {
                  if (!mcd.geometry) return;
                  try {
                      const props = mcd.properties || {};
                      const mcdNum = extractMCDNumber(props);
                      const parsed = parseMCDText(props._fullText || '');
                      const issuedDate = parseIssuedTime(props._fullText);
                      const issued = issuedDate ? formatDateToLocal(issuedDate) : (props.idp_filedate ? formatArcGISDate(props.idp_filedate) : 'Not specified');
                      const spcUrl = mcdSPCLink(mcdNum);
                      const expire = extractExpireTime(parsed.valid);

                      const geoJsonLayer = L.geoJSON(mcd, {
                          style: { color:'#00aaaa', fillColor:'#00aaaa', fillOpacity:0.15, weight:2, opacity:0.9, dashArray:'10, 5' }
                      }).addTo(map);
                      warningLayers.push(geoJsonLayer);

                      let bodyHtml = '';
                      if (parsed.area||parsed.concerning||parsed.summary||parsed.discussion||parsed.probability) {
                          if (parsed.area) bodyHtml += '<p style="margin:3px 0;"><strong>Area:</strong> ' + escapeHtml(parsed.area) + '</p>';
                          if (parsed.concerning) bodyHtml += '<p style="margin:3px 0;"><strong>Concerning:</strong> ' + escapeHtml(parsed.concerning) + '</p>';
                          if (parsed.summary) bodyHtml += '<div style="margin-top:8px;padding-top:8px;border-top:1px solid #ccc;"><strong>Summary:</strong><p style="margin:4px 0 0;font-size:0.9em;max-height:160px;overflow-y:auto;line-height:1.4;">' + escapeHtml(parsed.summary) + '</p></div>';
                          if (parsed.discussion) bodyHtml += '<div style="margin-top:8px;padding-top:8px;border-top:1px solid #ccc;"><strong>Discussion:</strong><p style="margin:4px 0 0;font-size:0.85em;max-height:200px;overflow-y:auto;line-height:1.4;">' + escapeHtml(parsed.discussion) + '</p></div>';
                      } else if (props._fullText) {
                          const pt = props._fullText.replace(/\s+/g,' ').trim();
                          if (pt) bodyHtml = '<p style="font-size:0.9em;max-height:200px;overflow-y:auto;">' + escapeHtml(pt.substring(0,800)) + (pt.length>800?'…':'') + '</p>';
                      } else {
                          const pt = (props.popupinfo||'').replace(/<[^>]+>/g,' ').replace(/\s+/g,' ').trim();
                          if (pt) bodyHtml = '<p style="font-size:0.9em;max-height:200px;overflow-y:auto;">' + escapeHtml(pt.substring(0,800)) + (pt.length>800?'…':'') + '</p>';
                      }

                      let popupContent = '<div style="color:#000;min-width:260px;max-width:460px;">' +
                          '<h3 style="margin-top:0;margin-bottom:6px;color:#006666;">MCD #' + mcdNum + '</h3>' +
                          '<p style="margin:3px 0;"><strong>Issued:</strong> ' + escapeHtml(issued) + '</p>';
                      if (expire) popupContent += '<p style="margin:3px 0;"><strong>Expires:</strong> ' + escapeHtml(formatExpireToLocal(expire, props._fullText)) + '</p>';
                      if (parsed.probability) popupContent += '<p style="margin:3px 0;color:#ff6600;font-weight:bold;">Probability of Watch Issuance: ' + escapeHtml(parsed.probability) + '</p>';
                      popupContent += bodyHtml + '<p style="margin-top:10px;padding-top:8px;border-top:1px solid #ccc;"><a href="' + spcUrl + '" target="_blank" style="color:#006666;font-weight:bold;">View full discussion on SPC ↗</a></p></div>';

                      geoJsonLayer.bindPopup(popupContent, { maxWidth:480, maxHeight:500 });
                      geoJsonLayer.bindTooltip('MCD #' + mcdNum + (parsed.concerning?' — '+parsed.concerning:'') + (parsed.area?' ('+parsed.area.substring(0,60)+(parsed.area.length>60?'…':'')+')':''), { sticky:true });
                      geoJsonLayer.on('mouseover', e => e.target.setStyle({ fillOpacity:0.3, weight:3 }));
                      geoJsonLayer.on('mouseout',  e => e.target.setStyle({ fillOpacity:0.15, weight:2 }));
                  } catch(error) { console.error('Error adding MCD to map:', error); }
              });
          }

          function getWarningColor(warningType, severity) {
              const t = (warningType||'').toLowerCase();
              if (t.includes('tornado warning')) return '#ff69b4';
              if (t.includes('tornado')&&t.includes('watch')) return '#ffff00';
              if (t.includes('thunderstorm warning')||t.includes('t-storm warning')||t.includes('tstorm warning')) return '#ff0000';
              if ((t.includes('thunderstorm')||t.includes('t-storm')||t.includes('tstorm'))&&t.includes('watch')) return '#aaaa00';
              if (severity==='Severe'||severity==='Extreme') return '#a52a2a';
              if (severity==='Moderate') return '#b25900';
              return '#666666';
          }

          function addWarningsToMap() {
              let added=0, skipped=0, fallback=0;
              const valid = warningsData.filter(w => w && w.severity !== 'Header' &&
                  ((w.geometry&&w.geometry.type)||(w.same&&w.same.length>0&&countyBoundaries)));
              const order = ['tornado warning','severe thunderstorm warning','tornado watch','severe thunderstorm watch'];
              valid.sort((a,b) => {
                  const at=(a.type||'').toLowerCase(), bt=(b.type||'').toLowerCase();
                  let ai=order.length, bi=order.length;
                  order.forEach((o,i) => { if(at.includes(o)) ai=i; if(bt.includes(o)) bi=i; });
                  return ai!==bi ? ai-bi : new Date(a.time||0)-new Date(b.time||0);
              });
              valid.forEach(warning => {
                  const color = getWarningColor(warning.type, warning.severity);
                  try {
                      if (warning.geometry&&warning.geometry.type) {
                          if (warning.geometry.type==='Polygon') { drawPolygon(warning.geometry.coordinates, warning, color); added++; }
                          else if (warning.geometry.type==='MultiPolygon') { warning.geometry.coordinates.forEach(pc=>drawPolygon(pc,warning,color)); added++; }
                          else { if(addCountyFallback(warning,color)){fallback++;added++;}else skipped++; }
                      } else if (warning.same&&warning.same.length>0&&countyBoundaries) {
                          if(addCountyFallback(warning,color)){fallback++;added++;}else skipped++;
                      }
                  } catch(e) { console.error('Error adding warning to map:', warning.type, e); skipped++; }
              });
              console.log('Map: ' + added + ' added, ' + skipped + ' skipped, ' + fallback + ' county fallback');
          }

          function bringSevereToFront() {
              warningLayers.forEach(l => { if (l.warningSeverity==='Severe') l.bringToFront(); });
          }

          function drawPolygon(coordinates, warning, color) {
              const latLngs = coordinates[0].map(c => [c[1],c[0]]);
              const polygon = L.polygon(latLngs, { color, fillColor:color, fillOpacity:0.3, weight:2, opacity:0.8 }).addTo(map);
              polygon.warningSeverity = warning.severity;
              warningLayers.push(polygon);
              const popup = '<div style="color:#000;min-width:250px;max-width:400px;">' +
                  '<h3 style="margin-top:0;">' + (warning.type||'Unknown') + '</h3>' +
                  '<p><strong>Severity:</strong> ' + (warning.severity||'Unknown') + '</p>' +
                  '<p><strong>Area:</strong> ' + (warning.area||'Unknown') + '</p>' +
                  '<p><strong>Issued:</strong> ' + formatTime(warning.time) + '</p>' +
                  '<p><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                  (warning.description?'<div style="margin-top:10px;padding-top:10px;border-top:1px solid #ccc;"><strong>Details:</strong><p style="font-size:0.9em;max-height:200px;overflow-y:auto;">' + warning.description + '</p></div>':'') +
                  '</div>';
              polygon.bindPopup(popup, { maxWidth:400, maxHeight:400 });
              polygon.bindTooltip((warning.type||'Warning') + ' - ' + (warning.area||'Unknown area'), { sticky:true });
          }

          function addCountyFallback(warning, color) {
              if (!countyBoundaries||!warning.same||warning.same.length===0) return false;
              let addedAny = false;
              warning.same.forEach(code => {
                  const fips = code.substring(1);
                  const county = countyBoundaries.features.find(f => f.id===fips||(f.properties&&f.properties.GEO_ID===fips));
                  if (county&&county.geometry) {
                      try {
                          const layer = L.geoJSON(county, {
                              style:{color,fillColor:color,fillOpacity:0.15,weight:2,opacity:0.6,dashArray:'5, 5'}
                          }).addTo(map);
                          warningLayers.push(layer);
                          const popup = '<div style="color:#000;min-width:250px;max-width:400px;">' +
                              '<h3 style="margin-top:0;">' + (warning.type||'Unknown') + '</h3>' +
                              '<p><strong>Severity:</strong> ' + (warning.severity||'Unknown') + '</p>' +
                              '<p><strong>Area:</strong> ' + (warning.area||'Unknown') + '</p>' +
                              '<p><strong>Issued:</strong> ' + formatTime(warning.time) + '</p>' +
                              '<p><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                              (warning.description?'<div style="margin-top:10px;padding-top:10px;border-top:1px solid #ccc;"><strong>Details:</strong><p style="font-size:0.9em;max-height:200px;overflow-y:auto;">' + warning.description + '</p></div>':'') +
                              '<p style="font-style:italic;font-size:0.85em;margin-top:10px;padding-top:10px;border-top:1px solid #ccc;">⚠️ Showing county boundary — exact warning polygon unavailable</p>' +
                              '</div>';
                          layer.bindPopup(popup, { maxWidth:400, maxHeight:400 });
                          layer.bindTooltip((warning.type||'Warning') + ' - ' + (warning.area||'Unknown') + ' (County)', { sticky:true });
                          addedAny = true;
                      } catch(e) { console.error('Error adding county boundary:', e); }
                  }
              });
              return addedAny;
          }

          function formatTime(t) {
              if (!t) return 'Not specified';
              const d = new Date(t);
              return isNaN(d.getTime()) ? t : d.toLocaleString();
          }

          function showDebugInfo() {
              const dbg = document.getElementById('debug-info');
              if (dbg.style.display === 'none') {
                  let info = 'Total warnings: ' + warningsData.length + '\nActive MCDs: ' + mesoscaleDiscussions.length + '\n\n';
                  warningsData.forEach((w,i) => {
                      info += (i+1) + '. ' + (w.type||'Unknown') + ' - ' + (w.area||'Unknown') + '\n';
                      info += '   Severity: ' + (w.severity||'Unknown') + '\n';
                      info += '   Geometry: ' + (w.geometry?w.geometry.type:'NONE') + '\n';
                      if (w.same&&w.same.length>0) info += '   SAME: ' + w.same.join(', ') + '\n';
                      info += '\n';
                  });
                  dbg.textContent = info;
                  dbg.style.display = 'block';
              } else {
                  dbg.style.display = 'none';
              }
          }

          function updateAllExpirationCountdowns() {
              document.querySelectorAll('[data-expires-timestamp]').forEach(el => {
                  const ts = parseInt(el.getAttribute('data-expires-timestamp'));
                  if (!ts) return;
                  const left = ts - Math.floor(Date.now()/1000);
                  if (left <= 0) {
                      el.textContent = 'EXPIRED'; el.classList.add('urgent');
                  } else {
                      const h=Math.floor(left/3600), m=Math.floor((left%3600)/60), s=left%60;
                      el.textContent = (h>0?h+'h ':'') + m+'m '+s+'s';
                      el.classList.remove('urgent','warning');
                      if (left<1800) el.classList.add('urgent');
                      else if (left<7200) el.classList.add('warning');
                  }
              });
          }

          function enrichMCDsWithText() {}
       </script>
    </head>
    <body>
       <h1 style="text-align: center;">Active Weather Warnings</h1>

       {{ if .WarningTypeCounts }}
       <div class="warning-types" id="top">
          {{ range .WarningTypeCounts }}
             <div class="warning-type{{ if eq .Severity "Severe" }} severe{{ else if eq .Severity "Moderate" }} moderate{{ else if eq .Severity "Extreme" }} severe{{ else if eq .Severity "Minor" }}{{ else }} watch{{ end }}">
               <a href="#{{ .Type | urlquery }}">{{ .Type }}</a>: <span class="type-count">{{ .Count }}</span>
             </div>
          {{ end }}
       </div>
       {{ end }}

       <div class="warning-types">
          <div class="warning-type" style="background-color:#00aaaa;">
             <a href="#mcd-container">Mesoscale Discussions</a>: <span id="mcd-count">-</span>
          </div>
       </div>

       <h4>Warnings: {{ .Counter }}</h4>
       <h4>Mesoscale Discussions: <span id="mcd-total-count">0</span></h4>
       <h4 id="last-updated">Last updated: <span id="last-updated-time">{{ .LastUpdated }}</span></h4>
       <div class="next-refresh">Next refresh in <span class="countdown">15s</span></div>

       <div id="map-section" style="margin-bottom:30px;">
          <button onclick="showDebugInfo()" style="margin-bottom:10px;padding:5px 10px;background-color:var(--header-bg);color:var(--text-color);border:1px solid var(--header-border);border-radius:3px;cursor:pointer;">Show Debug Info</button>
          <div id="debug-info" style="display:none;background-color:var(--summary-bg);padding:10px;margin-bottom:10px;border-radius:5px;font-family:monospace;font-size:0.9em;max-height:200px;overflow-y:auto;"></div>
          <div id="map"></div>
       </div>

       <div id="list-section">
          <div id="mcd-container" style="margin-bottom:20px;"></div>
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
                         <h2 class="warning-title" style="cursor:pointer;" onclick="zoomToWarning('{{ .ID }}')">{{ .Type }}</h2>
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

       <div class="map-legend" style="margin-top:30px;">
          <h3>Map Legend</h3>
          <div class="legend-item"><div class="legend-color" style="background-color:#ff69b4;"></div><span>Tornado Warning</span></div>
          <div class="legend-item"><div class="legend-color" style="background-color:#ff0000;"></div><span>Severe Thunderstorm Warning</span></div>
          <div class="legend-item"><div class="legend-color" style="background-color:#ffff00;"></div><span>Tornado Watch</span></div>
          <div class="legend-item"><div class="legend-color" style="background-color:#aaaa00;"></div><span>Severe Thunderstorm Watch</span></div>
          <div class="legend-item"><div class="legend-color" style="background-color:#00aaaa;border-style:dashed;"></div><span>Mesoscale Discussion</span></div>
          <div class="legend-item"><div class="legend-color" style="background-color:#a52a2a;"></div><span>Other Severe Warnings</span></div>
          <div class="legend-item"><div class="legend-color" style="background-color:#b25900;"></div><span>Moderate Warnings</span></div>
          <div style="margin-top:10px;padding-top:10px;border-top:1px solid var(--card-border);font-size:0.9em;color:#888;">
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
		Warnings                 []TemplateWarning
		LastUpdated              string
		Counter                  int
		WarningTypeCounts        []TypeCount
		WarningsJSON             template.JS
		MesoscaleDiscussionsJSON template.JS
		UpdatedAtUTC             int64
	}{
		Warnings:                 convertWarnings(warnings),
		LastUpdated:              time.Now().UTC().Format("Jan 2, 2006 at 03:04:01 UTC"),
		Counter:                  len(warnings),
		WarningTypeCounts:        sortedWarningTypeCounts(warnings),
		WarningsJSON:             template.JS(warningsJSON),
		MesoscaleDiscussionsJSON: template.JS("[]"),
		UpdatedAtUTC:             time.Now().UTC().Unix(),
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

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
	for _, w := range warnings {
		if w.Severity != "Header" {
			typeCounts[w.Type]++
			if cur := severityMap[w.Type]; cur == "" || getSeverityRank(w.Severity) > getSeverityRank(cur) {
				severityMap[w.Type] = w.Severity
			}
		}
	}
	return typeCounts, severityMap
}

func sortedWarningTypeCounts(warnings []fetcher.Warning) []TypeCount {
	typeCounts, severityMap := countWarningTypes(warnings)
	var result []TypeCount
	for wt, count := range typeCounts {
		result = append(result, TypeCount{Type: wt, Count: count, Priority: getWarningTypeRank(wt), Severity: severityMap[wt]})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].Count > result[j].Count
	})
	return result
}

// TemplateWarning wraps fetcher.Warning with additional template fields
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
	t := strings.ToLower(warningType)
	if strings.Contains(t, "tornado warning") {
		return 1
	}
	if strings.Contains(t, "thunderstorm warning") || strings.Contains(t, "t-storm warning") || strings.Contains(t, "tstorm warning") {
		return 2
	}
	if strings.Contains(t, "tornado") && strings.Contains(t, "watch") {
		return 3
	}
	if (strings.Contains(t, "thunderstorm") || strings.Contains(t, "t-storm") || strings.Contains(t, "tstorm")) && strings.Contains(t, "watch") {
		return 4
	}
	return 5
}

func convertWarnings(warnings []fetcher.Warning) []TemplateWarning {
	byType := make(map[string][]TemplateWarning)
	var types []string

	for _, w := range warnings {
		t := strings.ToLower(w.Type)
		var sc string
		switch {
		case strings.Contains(t, "tornado") && strings.Contains(t, "watch"):
			sc = "tornado-watch"
		case (strings.Contains(t, "thunderstorm") || strings.Contains(t, "t-storm") || strings.Contains(t, "tstorm")) && strings.Contains(t, "watch"):
			sc = "watch"
		case strings.Contains(t, "tornado warning"):
			sc = "tornado"
		case strings.Contains(t, "thunderstorm warning") || strings.Contains(t, "t-storm warning") || strings.Contains(t, "tstorm warning"):
			sc = "tstorm"
		default:
			sc = getSeverityClass(w.Severity)
		}

		tw := TemplateWarning{
			Warning:          w,
			SeverityClass:    sc,
			SeverityRank:     getSeverityRank(w.Severity),
			WarningTypeRank:  getWarningTypeRank(w.Type),
			LocalIssued:      formatToLocalTime(w.Time),
			LocalExpires:     formatToLocalTime(w.ExpiresTime),
			ExpiresTimestamp: getExpiresTimestamp(w.ExpiresTime),
			ID:               w.ID,
		}
		if _, exists := byType[w.Type]; !exists {
			types = append(types, w.Type)
		}
		byType[w.Type] = append(byType[w.Type], tw)
	}

	for wt := range byType {
		sort.Slice(byType[wt], func(i, j int) bool { return byType[wt][i].SeverityRank > byType[wt][j].SeverityRank })
	}

	sort.Slice(types, func(i, j int) bool {
		ri, rj := getWarningTypeRank(types[i]), getWarningTypeRank(types[j])
		if ri != rj {
			return ri < rj
		}
		mi, mj := 0, 0
		for _, w := range byType[types[i]] {
			if w.SeverityRank > mi {
				mi = w.SeverityRank
			}
		}
		for _, w := range byType[types[j]] {
			if w.SeverityRank > mj {
				mj = w.SeverityRank
			}
		}
		return mi > mj
	})

	var result []TemplateWarning
	for _, wt := range types {
		t := strings.ToLower(wt)
		var extra string
		switch {
		case strings.Contains(t, "tornado warning"):
			extra = "tornado-header"
		case strings.Contains(t, "tornado") && strings.Contains(t, "watch"):
			extra = "tornado-watch-header"
		case (strings.Contains(t, "thunderstorm") || strings.Contains(t, "t-storm") || strings.Contains(t, "tstorm")) && strings.Contains(t, "watch"):
			extra = "watch-header"
		case strings.Contains(t, "thunderstorm warning") || strings.Contains(t, "t-storm warning") || strings.Contains(t, "tstorm warning"):
			extra = "tstorm-header"
		}
		result = append(result, TemplateWarning{
			Warning:         fetcher.Warning{Type: wt, Severity: "Header"},
			SeverityClass:   "header",
			WarningTypeRank: getWarningTypeRank(wt),
			ExtraClass:      extra,
		})
		result = append(result, byType[wt]...)
	}
	return result
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
