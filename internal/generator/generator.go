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
   <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
   <title>US Weather Warnings</title>
   <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />
   <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
   <style>
      * {
         margin: 0;
         padding: 0;
         box-sizing: border-box;
      }
      html, body {
         height: 100%;
         width: 100%;
         overflow: hidden;
      }
      :root {
         --bg-color: #000000;
         --text-color: #ffffff;
         --text-muted: #888888;
         --card-bg: #1a1a1a;
         --card-border: #333333;
         --panel-bg: #0d0d0d;
         --status-bg: #111111;
         
         --tornado-color: #FF1493;
         --tornado-bg: #2d0a1f;
         --tstorm-color: #FF0000;
         --tstorm-bg: #2a0a0a;
         --tornado-watch-color: #FFFF00;
         --tornado-watch-bg: #2a2a0a;
         --watch-color: #FFA500;
         --watch-bg: #2a1a0a;
         --severe-color: #FF4444;
         --severe-bg: #2a0f0f;
         --moderate-color: #FFAA00;
         --moderate-bg: #2a1f0a;
         --mcd-color: #00FFFF;
         --mcd-bg: #0a2a2a;
         
         --countdown-urgent: #FF0000;
         --countdown-warning: #FFAA00;
         --countdown-ok: #00FF00;
      }
      body {
         font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
         background-color: var(--bg-color);
         color: var(--text-color);
         font-size: 18px;
         line-height: 1.4;
      }
      
      .status-bar {
         display: flex;
         align-items: center;
         justify-content: space-between;
         padding: 12px 20px;
         background: var(--status-bg);
         border-bottom: 1px solid #222;
         flex-shrink: 0;
      }
      .status-bar h1 {
         font-size: 22px;
         font-weight: 600;
         color: #fff;
         text-transform: uppercase;
         letter-spacing: 2px;
      }
      .status-summary {
         display: flex;
         gap: 20px;
         align-items: center;
      }
      .status-item {
         display: flex;
         align-items: center;
         gap: 8px;
         padding: 6px 14px;
         border-radius: 6px;
         font-size: 16px;
         font-weight: 600;
      }
      .status-item.tornado { background: var(--tornado-bg); border: 1px solid var(--tornado-color); color: var(--tornado-color); }
      .status-item.tstorm { background: var(--tstorm-bg); border: 1px solid var(--tstorm-color); color: var(--tstorm-color); }
      .status-item.watch { background: var(--watch-bg); border: 1px solid var(--watch-color); color: var(--watch-color); }
       .status-item.mcd { background: var(--mcd-bg); border: 1px solid var(--mcd-color); color: var(--mcd-color); }
      .status-item.sps { background: #1a1a2a; border: 1px solid #8866ff; color: #8866ff; }
      .status-item.mcd.active { animation: mcdPulse 2s infinite; }
      @keyframes mcdPulse {
         0%, 100% { box-shadow: 0 0 0 0 rgba(0, 255, 255, 0.4); }
         50% { box-shadow: 0 0 0 8px rgba(0, 255, 255, 0); }
      }
      .status-item .count { font-size: 20px; font-weight: 700; }
      
      .status-time {
         display: flex;
         align-items: center;
         gap: 15px;
         font-size: 14px;
         color: var(--text-muted);
      }
      .status-time .countdown { color: #fff; font-weight: 600; }
      
      .main-container {
         display: flex;
         height: calc(100vh - 60px);
      }
      
      .map-panel {
         flex: 0 0 60%;
         position: relative;
         border-right: 1px solid #222;
      }
      #map {
         height: 100%;
         width: 100%;
      }
      .leaflet-control-reset-map {
         background-color: var(--card-bg);
         color: var(--text-color);
         padding: 10px 14px;
         border-radius: 6px;
         border: 1px solid var(--card-border);
         cursor: pointer;
         font-size: 14px;
         font-family: inherit;
      }
      .leaflet-control-reset-map:hover { background-color: #252525; }
      
      .warning-panel {
         flex: 0 0 40%;
         display: flex;
         flex-direction: column;
         background: var(--panel-bg);
         overflow-y: auto;
         overflow-x: hidden;
         -webkit-overflow-scrolling: touch;
      }
      
      .mcd-section {
         flex-shrink: 0;
         padding: 15px;
         background: var(--mcd-bg);
         border-bottom: 1px solid var(--mcd-color);
      }
      .mcd-section h3 {
         font-size: 14px;
         text-transform: uppercase;
         letter-spacing: 1px;
         color: var(--mcd-color);
         margin-bottom: 10px;
      }
      .mcd-cards {
         display: flex;
         gap: 12px;
         overflow-x: auto;
         padding-bottom: 5px;
      }
      .mcd-cards::-webkit-scrollbar { height: 6px; }
      .mcd-cards::-webkit-scrollbar-track { background: #0a1a1a; }
      .mcd-cards::-webkit-scrollbar-thumb { background: var(--mcd-color); border-radius: 3px; }
      .mcd-card {
         flex-shrink: 0;
         background: rgba(0, 255, 255, 0.1);
         border: 1px solid var(--mcd-color);
         border-radius: 8px;
         padding: 12px 16px;
         min-width: 280px;
         cursor: pointer;
         transition: all 0.2s;
      }
      .mcd-card:hover { background: rgba(0, 255, 255, 0.2); transform: translateY(-2px); }
      .mcd-card-header {
         display: flex;
         justify-content: space-between;
         align-items: center;
         margin-bottom: 8px;
      }
      .mcd-card-header .mcd-num {
         font-size: 18px;
         font-weight: 700;
         color: var(--mcd-color);
      }
      .mcd-card-header .powi {
         font-size: 14px;
         font-weight: 600;
         color: #FFA500;
      }
      .mcd-card-area {
         font-size: 14px;
         color: #ccc;
         margin-bottom: 4px;
      }
      .mcd-card-concerning {
         font-size: 13px;
         color: #FFA500;
         font-weight: 500;
      }
      .mcd-card-link {
         display: inline-block;
         margin-top: 8px;
         font-size: 12px;
         color: var(--mcd-color);
         text-decoration: none;
      }
      .mcd-card-link:hover { text-decoration: underline; }
      
      .warnings-section {
         flex: 1;
         overflow-y: auto;
         padding: 15px;
         padding-bottom: 80px;
      }
      .warnings-section::-webkit-scrollbar { width: 8px; }
      .warnings-section::-webkit-scrollbar-track { background: #0a0a0a; }
      .warnings-section::-webkit-scrollbar-thumb { background: #333; border-radius: 4px; }
      
      .warning-type-header {
         padding: 10px 15px;
         border-radius: 8px 8px 0 0;
         margin-bottom: 0;
      }
      .warning-type-header.tornado { background: var(--tornado-bg); border: 2px solid var(--tornado-color); border-bottom: none; }
      .warning-type-header.tstorm { background: var(--tstorm-bg); border: 2px solid var(--tstorm-color); border-bottom: none; }
      .warning-type-header.tornado-watch { background: var(--tornado-watch-bg); border: 2px solid var(--tornado-watch-color); border-bottom: none; }
      .warning-type-header.watch { background: var(--watch-bg); border: 2px solid var(--watch-color); border-bottom: none; }
      .warning-type-header.severe { background: var(--severe-bg); border: 2px solid var(--severe-color); border-bottom: none; }
      .warning-type-header.moderate { background: var(--moderate-bg); border: 2px solid var(--moderate-color); border-bottom: none; }
      .warning-type-header h2 {
         font-size: 16px;
         text-transform: uppercase;
         letter-spacing: 1px;
         margin: 0;
      }
      .warning-type-header.tornado h2 { color: var(--tornado-color); }
      .warning-type-header.tstorm h2 { color: var(--tstorm-color); }
      .warning-type-header.tornado-watch h2 { color: var(--tornado-watch-color); }
      .warning-type-header.watch h2 { color: var(--watch-color); }
      .warning-type-header.severe h2 { color: var(--severe-color); }
      .warning-type-header.moderate h2 { color: var(--moderate-color); }
      
      .warning-card {
         padding: 16px;
         border-radius: 0 0 8px 8px;
         margin-bottom: 15px;
         border: 2px solid;
         border-top: none;
      }
      .warning-card.tornado { background: var(--tornado-bg); border-color: var(--tornado-color); }
      .warning-card.tstorm { background: var(--tstorm-bg); border-color: var(--tstorm-color); }
      .warning-card.tornado-watch { background: var(--tornado-watch-bg); border-color: var(--tornado-watch-color); }
      .warning-card.watch { background: var(--watch-bg); border-color: var(--watch-color); }
      .warning-card.severe { background: var(--severe-bg); border-color: var(--severe-color); }
      .warning-card.moderate { background: var(--moderate-bg); border-color: var(--moderate-color); }
      
      .warning-card-header {
         display: flex;
         justify-content: space-between;
         align-items: flex-start;
         margin-bottom: 10px;
      }
      .warning-card-header .severity-badge {
         font-size: 12px;
         font-weight: 600;
         padding: 4px 8px;
         border-radius: 4px;
         text-transform: uppercase;
      }
      .warning-card-header.tornado .severity-badge { background: var(--tornado-color); color: #000; }
      .warning-card-header.tstorm .severity-badge { background: var(--tstorm-color); color: #fff; }
      .warning-card-header.tornado-watch .severity-badge { background: var(--tornado-watch-color); color: #000; }
      .warning-card-header.watch .severity-badge { background: var(--watch-color); color: #000; }
      .warning-card-header.severe .severity-badge { background: var(--severe-color); color: #fff; }
      .warning-card-header.moderate .severity-badge { background: var(--moderate-color); color: #000; }
      
      .warning-card h3 {
         font-size: 18px;
         font-weight: 600;
         cursor: pointer;
         flex: 1;
         margin-right: 10px;
      }
      .warning-card h3:hover { text-decoration: underline; }
      .warning-card.tornado h3 { color: var(--tornado-color); }
      .warning-card.tstorm h3 { color: var(--tstorm-color); }
      .warning-card.tornado-watch h3 { color: var(--tornado-watch-color); }
      .warning-card.watch h3 { color: var(--watch-color); }
      .warning-card.severe h3 { color: var(--severe-color); }
      .warning-card.moderate h3 { color: var(--moderate-color); }
      
      .warning-card .area {
         font-size: 16px;
         font-weight: 500;
         color: #fff;
         margin-bottom: 10px;
      }
      .warning-card .description {
         font-size: 14px;
         color: #aaa;
         margin-bottom: 12px;
         max-height: 100px;
         overflow-y: auto;
      }
      .warning-card .times {
         display: flex;
         justify-content: space-between;
         align-items: center;
         font-size: 13px;
         color: #888;
         padding-top: 10px;
         border-top: 1px solid rgba(255,255,255,0.1);
      }
      .warning-card .countdown {
         font-size: 20px;
         font-weight: 700;
         font-family: 'Courier New', monospace;
      }
      .warning-card .countdown.urgent { color: var(--countdown-urgent); }
      .warning-card .countdown.warning { color: var(--countdown-warning); }
      .warning-card .countdown.ok { color: var(--countdown-ok); }
      
      .no-warnings {
         text-align: center;
         padding: 40px;
         color: #444;
         font-size: 20px;
      }
      
      @media (max-width: 1024px) {
         .main-container { flex-direction: column; height: calc(100vh - 60px); }
         .map-panel { flex: 0 0 45%; border-right: none; border-bottom: 1px solid #222; }
         .warning-panel { flex: 1; min-height: 0; overflow-y: auto; }
         .status-bar { flex-wrap: wrap; gap: 10px; }
         .status-bar h1 { font-size: 16px; }
      }
      
      @media (max-width: 600px) {
         body { font-size: 14px; }
         .status-bar {
            flex-direction: column;
            gap: 8px;
            padding: 10px;
         }
         .status-bar h1 { 
            font-size: 14px; 
            letter-spacing: 1px;
         }
         .status-summary {
            flex-wrap: wrap;
            justify-content: center;
            gap: 8px;
         }
         .status-item {
            padding: 4px 8px;
            font-size: 12px;
         }
         .status-item .count { font-size: 14px; }
         .status-time {
            font-size: 11px;
         }
         .main-container {
            flex-direction: column;
            height: calc(100vh - 60px);
         }
         .map-panel {
            flex: 0 0 45%;
            border-right: none;
            border-bottom: 1px solid #222;
         }
         .map-panel #map {
            touch-action: none;
         }
         .warning-panel {
            flex: 1;
            min-height: 0;
            overflow-y: auto;
            -webkit-overflow-scrolling: touch;
         }
         .mcd-section {
            padding: 10px;
         }
         .mcd-section h3 {
            font-size: 12px;
         }
         .mcd-card {
            min-width: 220px;
            padding: 10px;
         }
         .mcd-card-header .mcd-num {
            font-size: 14px;
         }
         .warning-card {
            padding: 12px;
         }
         .warning-card h3 {
            font-size: 14px;
         }
         .warning-card .area {
            font-size: 13px;
         }
         .warning-card .description {
            font-size: 12px;
            max-height: 60px;
         }
         .warning-card .countdown {
            font-size: 16px;
         }
         .warning-type-header h2 {
            font-size: 12px;
         }
      }
    </style>
   <script>
      let map;
      let radarLayer;
      let warningsData = {{ .WarningsJSON }};
      let countyBoundaries = null;
      let warningLayers = [];
      let mesoscaleDiscussions = [];
      let validMCDs = [];
      let lastUpdateTime = Date.now();

      async function fetchUpdatedWarnings() {
         try {
            const response = await fetch('warnings.json?_=' + Date.now());
            if (!response.ok) throw new Error('Failed to read warnings.json: ' + response.status);

            const payload = await response.json();
            console.log('[poll] ' + (payload.warnings || []).length + ' warnings from warnings.json, updated ' + payload.lastUpdated);

            const now = Date.now();
            warningsData = (payload.warnings || []).filter(w =>
               !w.expiresTime || new Date(w.expiresTime).getTime() > now
            );

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
         updateWarningTypeCounts();
      }

      function updateWarningTypeCounts() {
         const counts = { tornado: 0, tstorm: 0, watch: 0, sps: 0 };
         warningsData.forEach(w => {
            if (!w.type) return;
            const t = w.type.toLowerCase();
            if (t.includes('tornado warning')) counts.tornado++;
            else if (t.includes('thunderstorm warning')) counts.tstorm++;
            else if (t.includes('watch')) counts.watch++;
            else if (t.includes('special weather statement')) counts.sps++;
         });
         
         const tornadoEl = document.getElementById('count-tornado');
         const tstormEl = document.getElementById('count-tstorm');
         const watchEl = document.getElementById('count-watch');
         const spsEl = document.getElementById('count-sps');
         if (tornadoEl) tornadoEl.textContent = counts.tornado;
         if (tstormEl) tstormEl.textContent = counts.tstorm;
         if (watchEl) watchEl.textContent = counts.watch;
         if (spsEl) spsEl.textContent = counts.sps;
         
         const mcdCountEl = document.getElementById('mcd-count');
         const mcdStatusEl = document.getElementById('mcd-status');
         if (validMCDs) {
            if (mcdCountEl) mcdCountEl.textContent = validMCDs.length;
            if (mcdStatusEl) {
               if (validMCDs.length > 0) {
                  mcdStatusEl.classList.add('active');
               } else {
                  mcdStatusEl.classList.remove('active');
               }
            }
         }
      }

      function addMesoscaleDiscussionsToList() {
         const container = document.getElementById('mcd-cards-container');
         const mcdSection = container ? container.closest('.mcd-section') : null;
         if (!container) return;

         validMCDs = mesoscaleDiscussions.filter(mcd => extractMCDNumber(mcd.properties || {}) !== '????');

         const mcdCountEl = document.getElementById('mcd-count');
         if (mcdCountEl) mcdCountEl.textContent = validMCDs.length;
         const mcdStatusEl = document.getElementById('mcd-status');
         if (mcdStatusEl) {
            if (validMCDs.length > 0) {
               mcdStatusEl.classList.add('active');
            } else {
               mcdStatusEl.classList.remove('active');
            }
         }

         if (validMCDs.length === 0) { 
            if (mcdSection) mcdSection.style.display = 'none';
            return; 
         }
         
         if (mcdSection) mcdSection.style.display = 'block';

         let html = '';
         validMCDs.forEach((mcd, index) => {
            const props = mcd.properties || {};
            const mcdNum = extractMCDNumber(props);
            const parsed = parseMCDText(props._fullText || '');
            const issuedDate = parseIssuedTime(props._fullText);
            const issued = issuedDate ? formatDateToLocal(issuedDate) : (props.idp_filedate ? formatArcGISDate(props.idp_filedate) : 'Not specified');
            const spcUrl = mcdSPCLink(mcdNum);
            const expire = extractExpireTime(parsed.valid);

            html += '<div class="mcd-card" onclick="zoomToMCD(' + index + ')">';
            html += '<div class="mcd-card-header">';
            html += '<span class="mcd-num">MCD #' + mcdNum + '</span>';
            if (parsed.probability) html += '<span class="powi">POWI: ' + escapeHtml(parsed.probability) + '</span>';
            html += '</div>';
            if (parsed.area) html += '<div class="mcd-card-area">' + escapeHtml(parsed.area.substring(0, 60)) + (parsed.area.length > 60 ? '...' : '') + '</div>';
            if (parsed.concerning) html += '<div class="mcd-card-concerning">' + escapeHtml(parsed.concerning) + '</div>';
            html += '<a href="' + spcUrl + '" target="_blank" class="mcd-card-link" onclick="event.stopPropagation();">View on SPC ↗</a>';
            html += '</div>';
         });
         container.innerHTML = html;
         updateAllExpirationCountdowns();
      }

      function escapeHtml(str) {
         return String(str).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
      }

      function zoomToMCD(index) {
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
         return '<div class="warning-card ' + severityClass + '" data-warning-id="' + warning.id + '">' +
            '<div class="warning-card-header ' + severityClass + '">' +
               '<h3 onclick="zoomToWarning(\'' + warning.id + '\')">' + (warning.type||'') + '</h3>' +
               '<span class="severity-badge">' + (warning.severity||'') + '</span>' +
            '</div>' +
            '<div class="area">' + (warning.area||'') + '</div>' +
            '<div class="description">' + (warning.description||'') + '</div>' +
            '<div class="times">' +
               '<span>Expires: ' + parseISOTime(warning.expiresTime) + '</span>' +
               '<span class="expiration-countdown" data-expires-timestamp="' + expiresTimestamp + '"></span>' +
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
         
         if (sortedTypes.length === 0) {
            return '<div class="no-warnings">No active weather warnings</div>';
         }
         
         let html = '';
         sortedTypes.forEach(type => {
            const sc = getWarningSeverityClass(byType[type][0]);
            html += '<div class="warning-type-header ' + sc + '" id="' + encodeURIComponent(type) + '"><h2>' + type + ' (' + byType[type].length + ')</h2></div>';
            byType[type].forEach(w => { html += renderWarningCard(w); });
         });
         return html;
      }

      function updateListView(warnings) {
         const listSection = document.getElementById('warnings-list');
         if (!listSection) return;
         
         if (warnings.length === 0) {
            listSection.innerHTML = '<div class="no-warnings">No active weather warnings</div>';
         } else {
            listSection.innerHTML = renderWarningsList(warnings);
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

         const refreshInterval = 15000;
         let refreshTime = 15;
         const countdownElements = document.querySelectorAll('.countdown');
         
         function updateCountdown() {
            countdownElements.forEach(el => el.textContent = refreshTime + 's');
         }
         
         setInterval(function() {
            refreshTime--;
            if (refreshTime <= 0) {
               countdownElements.forEach(el => el.textContent = '...');
            } else {
               updateCountdown();
            }
         }, 1000);
         updateCountdown();

         setInterval(function() {
            refreshTime = 15;
            updateCountdown();
            fetchUpdatedWarnings();
         }, refreshInterval);
         fetchUpdatedWarnings();

         updateAllExpirationCountdowns();

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
               updateWarningTypeCounts();
            }
            updateAllExpirationCountdowns();
         }, 1000);
      };

      function zoomToWarning(warningId) {
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
         map = L.map('map', { zoomControl: true }).setView([39.8283, -98.5795], 4);

         const savedMapState = localStorage.getItem('mapState');
         if (savedMapState) {
            try { const s = JSON.parse(savedMapState); map.setView([s.lat, s.lng], s.zoom); } catch(e) {}
         }

         L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
            attribution: '&copy; OpenStreetMap &copy; CARTO',
            subdomains: 'abcd', maxZoom: 20
         }).addTo(map);

         radarLayer = L.tileLayer('https://mesonet.agron.iastate.edu/c/tile.py/1.0.0/ridge::N0Q-0/{z}/{x}/{y}.png', {
            maxZoom: 16,
            attribution: 'Radar &copy; IEM'
         });
         radarLayer.addTo(map);

         const overlays = { "Radar": radarLayer };
         L.control.layers(null, overlays, { collapsed: false, autoZIndex: false }).addTo(map);

         const initialView = [39.8283, -98.5795], initialZoom = 4;
         L.Control.ResetMap = L.Control.extend({
            onAdd: function(map) {
               const btn = L.DomUtil.create('button', 'leaflet-control-reset-map');
               btn.innerHTML = '⟲ Reset';
               btn.title = 'Reset to full US view';
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
                  style: { color:'#00FFFF', fillColor:'#00FFFF', fillOpacity:0.15, weight:2, opacity:0.9, dashArray:'10, 5' }
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

               let popupContent = '<div style="color:#000;min-width:200px;max-width:280px;font-size:13px;">' +
                  '<h3 style="margin:0 0 6px 0;font-size:14px;color:#006666;">MCD #' + mcdNum + '</h3>' +
                  '<p style="margin:3px 0;"><strong>Issued:</strong> ' + escapeHtml(issued) + '</p>';
               if (expire) popupContent += '<p style="margin:3px 0;"><strong>Expires:</strong> ' + escapeHtml(formatExpireToLocal(expire, props._fullText)) + '</p>';
               if (parsed.probability) popupContent += '<p style="margin:3px 0;color:#ff6600;font-weight:bold;">POWI: ' + escapeHtml(parsed.probability) + '</p>';
               popupContent += bodyHtml + '<p style="margin-top:8px;padding-top:8px;border-top:1px solid #ccc;"><a href="' + spcUrl + '" target="_blank" style="color:#006666;font-size:12px;">View on SPC ↗</a></p></div>';

               geoJsonLayer.bindPopup(popupContent, { maxWidth:300, maxHeight:350 });
               geoJsonLayer.bindTooltip('MCD #' + mcdNum + (parsed.concerning?' — '+parsed.concerning:'') + (parsed.area?' ('+parsed.area.substring(0,60)+(parsed.area.length>60?'…':'')+')':''), { sticky:true });
               geoJsonLayer.on('mouseover', e => e.target.setStyle({ fillOpacity:0.3, weight:3 }));
               geoJsonLayer.on('mouseout',  e => e.target.setStyle({ fillOpacity:0.15, weight:2 }));
            } catch(error) { console.error('Error adding MCD to map:', error); }
         });
      }

      function getWarningColor(warningType, severity) {
         const t = (warningType||'').toLowerCase();
         if (t.includes('tornado warning')) return '#FF1493';
         if (t.includes('tornado')&&t.includes('watch')) return '#FFFF00';
         if (t.includes('thunderstorm warning')||t.includes('t-storm warning')||t.includes('tstorm warning')) return '#FF0000';
         if ((t.includes('thunderstorm')||t.includes('t-storm')||t.includes('tstorm'))&&t.includes('watch')) return '#FFA500';
         if (severity==='Severe'||severity==='Extreme') return '#FF4444';
         if (severity==='Moderate') return '#FFAA00';
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
         const popup = '<div style="color:#000;min-width:200px;max-width:280px;font-size:13px;">' +
            '<h3 style="margin:0 0 8px 0;font-size:14px;">' + (warning.type||'Unknown') + '</h3>' +
            '<p style="margin:3px 0;"><strong>Severity:</strong> ' + (warning.severity||'Unknown') + '</p>' +
            '<p style="margin:3px 0;"><strong>Area:</strong> ' + (warning.area||'Unknown') + '</p>' +
            '<p style="margin:3px 0;"><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
            (warning.description?'<div style="margin-top:8px;padding-top:8px;border-top:1px solid #ccc;font-size:12px;"><strong>Details:</strong><p style="margin:4px 0 0;font-size:11px;max-height:80px;overflow-y:auto;line-height:1.3;">' + warning.description + '</p></div>':'') +
            '</div>';
         polygon.bindPopup(popup, { maxWidth:300, maxHeight:250 });
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
                     '<p style="margin:3px 0;"><strong>Severity:</strong> ' + (warning.severity||'Unknown') + '</p>' +
                     '<p style="margin:3px 0;"><strong>Area:</strong> ' + (warning.area||'Unknown') + '</p>' +
                     '<p style="margin:3px 0;"><strong>Expires:</strong> ' + formatTime(warning.expiresTime) + '</p>' +
                     (warning.description?'<div style="margin-top:8px;padding-top:8px;border-top:1px solid #ccc;font-size:12px;"><strong>Details:</strong><p style="margin:4px 0 0;font-size:11px;max-height:80px;overflow-y:auto;line-height:1.3;">' + warning.description + '</p></div>':'') +
                     '<p style="font-style:italic;font-size:11px;margin-top:8px;padding-top:8px;border-top:1px solid #ccc;">⚠️ County boundary only</p>' +
                     '</div>';
                  layer.bindPopup(popup, { maxWidth:300, maxHeight:250 });
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

      function updateAllExpirationCountdowns() {
         document.querySelectorAll('[data-expires-timestamp]').forEach(el => {
            const ts = parseInt(el.getAttribute('data-expires-timestamp'));
            if (!ts) return;
            const left = ts - Math.floor(Date.now()/1000);
            if (left <= 0) {
               el.textContent = 'EXPIRED';
               el.classList.remove('ok', 'warning');
               el.classList.add('urgent');
            } else {
               const h=Math.floor(left/3600), m=Math.floor((left%3600)/60), s=left%60;
               el.textContent = (h>0?h+'h ':'') + m+'m '+s+'s';
               el.classList.remove('urgent', 'warning');
               if (left<1800) el.classList.add('urgent');
               else if (left<7200) el.classList.add('warning');
               else el.classList.add('ok');
            }
         });
      }
   </script>
</head>
<body>
   <div class="status-bar">
      <h1>US Weather Warnings</h1>
      <div class="status-summary">
         <div class="status-item tornado">
            <span>⚡</span>
            <span>Tornado</span>
            <span class="count" id="count-tornado">0</span>
         </div>
         <div class="status-item tstorm">
            <span>🔴</span>
            <span>T-Storm</span>
            <span class="count" id="count-tstorm">0</span>
         </div>
         <div class="status-item watch">
            <span>👁</span>
            <span>Watches</span>
            <span class="count" id="count-watch">0</span>
         </div>
         <div class="status-item sps">
            <span>📋</span>
            <span>SPS</span>
            <span class="count" id="count-sps">0</span>
         </div>
         <div class="status-item mcd" id="mcd-status">
            <span>MCDs</span>
            <span class="count" id="mcd-count">0</span>
         </div>
      </div>
      <div class="status-time">
         <span>Updated: <span id="last-updated-time">{{ .LastUpdated }}</span></span>
         <span>Refresh: <span class="countdown">15s</span></span>
      </div>
   </div>

   <div class="main-container">
      <div class="map-panel">
         <div id="map"></div>
      </div>

      <div class="warning-panel">
         <div class="mcd-section">
            <h3>Mesoscale Discussions</h3>
            <div class="mcd-cards" id="mcd-cards-container">
               <div style="color:#444;font-size:14px;">Loading MCDs...</div>
            </div>
         </div>

         <div class="warnings-section" id="warnings-list">
            <div class="no-warnings">No active weather warnings</div>
         </div>
      </div>
   </div>
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
