package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/httpclient"
)

type musicPlay struct {
	client  *http.Client
	baseURL string
}

type itunesSearchResponse struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		TrackName      string `json:"trackName"`
		ArtistName     string `json:"artistName"`
		CollectionName string `json:"collectionName"`
		PreviewURL     string `json:"previewUrl"`
		TrackViewURL   string `json:"trackViewUrl"`
		ArtworkURL100  string `json:"artworkUrl100"`
	} `json:"results"`
}

type musicTrack struct {
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	PreviewURL string `json:"preview_url"`
	TrackURL   string `json:"track_url"`
	ArtworkURL string `json:"artwork_url"`
}

func NewMusicPlay() ports.ToolExecutor {
	return newMusicPlay(nil, "https://itunes.apple.com")
}

func newMusicPlay(client *http.Client, baseURL string) *musicPlay {
	if client == nil {
		client = httpclient.New(20*time.Second, nil)
	}
	return &musicPlay{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (t *musicPlay) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "music_play",
		Version:  "1.0.0",
		Category: "media",
		Tags:     []string{"music", "audio", "player"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"text/html"},
		},
	}
}

func (t *musicPlay) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "music_play",
		Description: `Recommend and play music previews by mood or request.

Uses:
- Open-source player: APlayer (MIT) via CDN
- Free API: iTunes Search API (no auth required)

Returns a playable HTML playlist (APlayer) with preview URLs.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"mood": {
					Type:        "string",
					Description: "User emotion or vibe, e.g. happy, sad, focus, relax, anxious.",
				},
				"request": {
					Type:        "string",
					Description: "Explicit user request (artist, genre, activity). Takes priority over mood.",
				},
				"limit": {
					Type:        "integer",
					Description: "Number of tracks to return (1-10, default 6).",
				},
				"country": {
					Type:        "string",
					Description: "iTunes storefront country code (default US).",
				},
			},
		},
	}
}

func (t *musicPlay) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	mood := strings.TrimSpace(stringArg(call.Arguments, "mood"))
	request := strings.TrimSpace(stringArg(call.Arguments, "request"))
	query := strings.TrimSpace(request)
	if query == "" {
		query = moodQuery(mood)
	}
	if query == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: mood or request is required.",
			Error:   fmt.Errorf("missing mood or request"),
		}, nil
	}

	limit := int(uint64Arg(call.Arguments, "limit"))
	if limit == 0 {
		limit = 6
	}
	if limit > 10 {
		limit = 10
	}
	if limit < 1 {
		limit = 1
	}

	country := strings.ToUpper(strings.TrimSpace(stringArg(call.Arguments, "country")))
	if country == "" {
		country = "US"
	}

	searchURL, err := t.buildSearchURL(ctx, query, limit, country)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	resp, err := t.client.Do(searchURL)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Error: iTunes API returned status %d.", resp.StatusCode),
			Error:   fmt.Errorf("itunes search failed"),
		}, nil
	}

	var payload itunesSearchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	tracks := make([]musicTrack, 0, len(payload.Results))
	for _, item := range payload.Results {
		if strings.TrimSpace(item.PreviewURL) == "" {
			continue
		}
		tracks = append(tracks, musicTrack{
			Title:      item.TrackName,
			Artist:     item.ArtistName,
			Album:      item.CollectionName,
			PreviewURL: item.PreviewURL,
			TrackURL:   item.TrackViewURL,
			ArtworkURL: item.ArtworkURL100,
		})
	}

	content := fmt.Sprintf("Matched %d tracks for %q. Open-source player: APlayer (MIT). Free API: iTunes Search API.", len(tracks), query)
	result := &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"query":   query,
			"mood":    mood,
			"request": request,
			"player": map[string]any{
				"name":     "APlayer",
				"license":  "MIT",
				"homepage": "https://aplayer.js.org/",
				"cdn_js":   "https://unpkg.com/aplayer/dist/APlayer.min.js",
				"cdn_css":  "https://unpkg.com/aplayer/dist/APlayer.min.css",
			},
			"api": map[string]any{
				"name": "iTunes Search API",
				"url":  "https://developer.apple.com/library/archive/documentation/AudioVideo/Conceptual/iTuneSearchAPI/index.html",
			},
			"tracks": tracks,
		},
	}

	if len(tracks) == 0 {
		return result, nil
	}

	html := buildMusicPlayerHTML(query, tracks)
	encoded := base64.StdEncoding.EncodeToString([]byte(html))
	attachment := ports.Attachment{
		Name:           fmt.Sprintf("%s.html", safeMusicFilename(query)),
		MediaType:      "text/html",
		Data:           encoded,
		Description:    fmt.Sprintf("Music player for %s", query),
		Kind:           "artifact",
		Format:         "html",
		PreviewProfile: "document.html",
		PreviewAssets: []ports.AttachmentPreviewAsset{
			{
				AssetID:     "live-preview",
				Label:       "HTML preview",
				MimeType:    "text/html",
				CDNURL:      fmt.Sprintf("data:text/html;base64,%s", encoded),
				PreviewType: "iframe",
			},
		},
	}
	result.Attachments = map[string]ports.Attachment{attachment.Name: attachment}

	return result, nil
}

func (t *musicPlay) buildSearchURL(ctx context.Context, query string, limit int, country string) (*http.Request, error) {
	endpoint := fmt.Sprintf("%s/search", t.baseURL)
	params := url.Values{}
	params.Set("term", query)
	params.Set("media", "music")
	params.Set("entity", "song")
	params.Set("limit", fmt.Sprintf("%d", limit))
	if country != "" {
		params.Set("country", country)
	}
	searchURL := fmt.Sprintf("%s?%s", endpoint, params.Encode())
	return http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
}

func moodQuery(mood string) string {
	normalized := strings.ToLower(strings.TrimSpace(mood))
	switch {
	case normalized == "":
		return ""
	case strings.Contains(normalized, "happy") || strings.Contains(normalized, "joy") || strings.Contains(normalized, "upbeat") || strings.Contains(normalized, "开心"):
		return "feel good pop"
	case strings.Contains(normalized, "sad") || strings.Contains(normalized, "blue") || strings.Contains(normalized, "伤心"):
		return "sad acoustic"
	case strings.Contains(normalized, "focus") || strings.Contains(normalized, "study") || strings.Contains(normalized, "专注"):
		return "lofi focus"
	case strings.Contains(normalized, "relax") || strings.Contains(normalized, "chill") || strings.Contains(normalized, "放松"):
		return "chill ambient"
	case strings.Contains(normalized, "angry") || strings.Contains(normalized, "rage") || strings.Contains(normalized, "愤怒"):
		return "rock workout"
	case strings.Contains(normalized, "anxious") || strings.Contains(normalized, "stress") || strings.Contains(normalized, "焦虑"):
		return "calm piano"
	case strings.Contains(normalized, "romance") || strings.Contains(normalized, "love") || strings.Contains(normalized, "浪漫"):
		return "romantic ballad"
	default:
		return fmt.Sprintf("%s music", normalized)
	}
}

func buildMusicPlayerHTML(query string, tracks []musicTrack) string {
	audioPayload := make([]map[string]string, 0, len(tracks))
	for _, track := range tracks {
		audioPayload = append(audioPayload, map[string]string{
			"name":   track.Title,
			"artist": track.Artist,
			"url":    track.PreviewURL,
			"cover":  track.ArtworkURL,
		})
	}
	audioJSON, _ := json.Marshal(audioPayload)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Music Player - %s</title>
  <link rel="stylesheet" href="https://unpkg.com/aplayer/dist/APlayer.min.css" />
  <style>
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0f172a;
      color: #e2e8f0;
      padding: 24px;
    }
    h1 {
      font-size: 20px;
      margin-bottom: 8px;
    }
    .subtitle {
      font-size: 14px;
      color: #94a3b8;
      margin-bottom: 20px;
    }
    .player {
      max-width: 640px;
    }
  </style>
</head>
<body>
  <div class="player">
    <h1>Music for %s</h1>
    <div class="subtitle">Powered by iTunes Search API + APlayer (MIT).</div>
    <div id="aplayer"></div>
  </div>

  <script src="https://unpkg.com/aplayer/dist/APlayer.min.js"></script>
  <script>
    const playlist = %s;
    const player = new APlayer({
      container: document.getElementById('aplayer'),
      audio: playlist
    });
    player.play();
  </script>
</body>
</html>`, htmlEscape(query), htmlEscape(query), string(audioJSON))
}

func htmlEscape(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(text)
}

func safeMusicFilename(title string) string {
	safe := strings.Map(func(r rune) rune {
		if r == ' ' || r == '_' || r == '-' {
			return '_'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, strings.TrimSpace(title))
	if safe == "" {
		return "music_player"
	}
	return safe
}
