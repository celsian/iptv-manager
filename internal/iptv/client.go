package iptv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/celsian/iptv-manager/internal/config"
)

type Channel struct {
	Title   string `json:"title"`
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
	Group   string `json:"group,omitempty"`
}

type Client struct {
	cfg        *config.Manager
	httpClient *http.Client
}

func NewClient(cfg *config.Manager) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{},
	}
}

func (c *Client) Search(playlist, searchTerm string) ([]Channel, error) {
	data := url.Values{
		"jxt": {"4"},
		"jxw": {"sch"},
		"s":   {playlist},
		"c":   {searchTerm},
	}

	jsonBody, err := c.requestJSON(data)
	if err != nil {
		return nil, err
	}

	return c.parseChannels(jsonBody)
}

func (c *Client) GetPlaylists() ([]string, error) {
	cfg := c.cfg.Get()

	data := url.Values{
		"jxt": {"4"},
		"jxw": {"sch"},
	}

	req, err := http.NewRequest("POST", cfg.IPTV.APIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", cfg.IPTV.UID, cfg.IPTV.Pass))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, err
	}

	return c.parsePlaylists(jsonBody)
}

func (c *Client) parsePlaylists(jsonBody map[string]interface{}) ([]string, error) {
	var playlists []string

	fs, ok := jsonBody["Fs"].([]interface{})
	if !ok || len(fs) < 2 {
		return playlists, nil
	}

	second, ok := fs[1].([]interface{})
	if !ok || len(second) < 2 {
		return playlists, nil
	}

	nested, ok := second[1].([]interface{})
	if !ok || len(nested) < 2 {
		return playlists, nil
	}

	after, ok := nested[1].([]interface{})
	if !ok || len(after) < 2 {
		return playlists, nil
	}

	html, ok := after[1].(string)
	if !ok {
		return playlists, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	doc.Find("select option").Each(func(i int, s *goquery.Selection) {
		val, exists := s.Attr("value")
		if exists && val != "" {
			playlists = append(playlists, val)
		}
	})

	return playlists, nil
}

func (c *Client) Toggle(playlist, channelID string, enable bool) error {
	data := url.Values{
		"jxt": {"4"},
		"jxw": {"s"},
		"s":   {playlist},
		"c":   {channelID},
	}

	if enable {
		data.Set("a", "1")
	} else {
		data.Set("a", "0")
	}

	_, err := c.requestJSON(data)
	return err
}

func (c *Client) GetChannelURL(channelID string) (string, error) {
	cfg := c.cfg.Get()
	baseURL := strings.TrimSuffix(cfg.IPTV.APIAddress, "/stalker_portal/server/load.php")
	streamURL := fmt.Sprintf("%s/stalker_portal/streaming/", baseURL)

	data := url.Values{
		"jxt": {"5"},
		"jxw": {"play"},
		"c":   {channelID},
	}

	req, err := http.NewRequest("POST", cfg.IPTV.APIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", cfg.IPTV.UID, cfg.IPTV.Pass))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return "", err
	}

	if cmd, ok := jsonBody["cmd"].(string); ok {
		if strings.HasPrefix(cmd, "http") {
			return cmd, nil
		}
		return streamURL + cmd, nil
	}

	return "", fmt.Errorf("unable to get stream URL")
}

func (c *Client) requestJSON(data url.Values) (map[string]interface{}, error) {
	cfg := c.cfg.Get()

	req, err := http.NewRequest("POST", cfg.IPTV.APIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", cfg.IPTV.UID, cfg.IPTV.Pass))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, err
	}

	return jsonBody, nil
}

func (c *Client) parseChannels(jsonBody map[string]interface{}) ([]Channel, error) {
	var channels []Channel

	fs, ok := jsonBody["Fs"].([]interface{})
	if !ok || len(fs) < 2 {
		return channels, nil
	}

	second, ok := fs[1].([]interface{})
	if !ok || len(second) < 2 {
		return channels, nil
	}

	nested, ok := second[1].([]interface{})
	if !ok || len(nested) < 2 {
		return channels, nil
	}

	after, ok := nested[1].([]interface{})
	if !ok || len(after) < 2 {
		return channels, nil
	}

	html, ok := after[1].(string)
	if !ok {
		return channels, nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		input := s.Find("input[type=checkbox]")
		title := strings.TrimSpace(s.Find("span").First().Text())
		group := strings.TrimSpace(s.Find("div.sub").Text())

		if input.Length() > 0 {
			id, _ := input.Attr("id")
			_, checked := input.Attr("checked")

			channels = append(channels, Channel{
				Title:   title,
				ID:      id,
				Enabled: checked,
				Group:   group,
			})
		}
	})

	// Sort by group, then by title
	sort.Slice(channels, func(i, j int) bool {
		if channels[i].Group != channels[j].Group {
			return strings.ToLower(channels[i].Group) < strings.ToLower(channels[j].Group)
		}
		return strings.ToLower(channels[i].Title) < strings.ToLower(channels[j].Title)
	})

	return channels, nil
}

func (c *Client) GetEnabledChannels(playlist string) ([]Channel, error) {
	cfg := c.cfg.Get()

	data := url.Values{
		"jxt": {"4"},
		"jxw": {"sch"},
		"s":   {playlist},
		"c":   {""},
	}

	req, err := http.NewRequest("POST", cfg.IPTV.APIAddress, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("uid=%s; pass=%s", cfg.IPTV.UID, cfg.IPTV.Pass))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var jsonBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonBody); err != nil {
		return nil, err
	}

	allChannels, err := c.parseChannels(jsonBody)
	if err != nil {
		return nil, err
	}

	var enabled []Channel
	for _, ch := range allChannels {
		if ch.Enabled {
			enabled = append(enabled, ch)
		}
	}

	return enabled, nil
}
