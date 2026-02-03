package emby

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/celsian/iptv-manager/internal/config"
)

type ScheduleTask struct {
	ID  string `json:"Id"`
	Key string `json:"Key"`
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

func (c *Client) RefreshGuide() error {
	cfg := c.cfg.Get()

	if cfg.Emby.APIAddress == "" || cfg.Emby.APIKey == "" {
		return fmt.Errorf("emby configuration not set")
	}

	scheduledTasksURL := fmt.Sprintf("%s/emby/ScheduledTasks?api_key=%s", cfg.Emby.APIAddress, cfg.Emby.APIKey)
	response, err := c.httpClient.Get(scheduledTasksURL)
	if err != nil {
		return fmt.Errorf("error getting scheduled tasks: %w", err)
	}
	defer response.Body.Close()

	var scheduledTasks []ScheduleTask
	if err := json.NewDecoder(response.Body).Decode(&scheduledTasks); err != nil {
		return fmt.Errorf("error decoding scheduled tasks: %w", err)
	}

	var refreshGuideID string
	for _, task := range scheduledTasks {
		if task.Key == "RefreshGuide" {
			refreshGuideID = task.ID
			break
		}
	}

	if refreshGuideID == "" {
		return fmt.Errorf("RefreshGuide task not found")
	}

	triggerTaskURL := fmt.Sprintf("%s/emby/ScheduledTasks/Running/%s?api_key=%s", cfg.Emby.APIAddress, refreshGuideID, cfg.Emby.APIKey)
	response, err = c.httpClient.Post(triggerTaskURL, "", nil)
	if err != nil {
		return fmt.Errorf("error triggering guide refresh: %w", err)
	}
	defer response.Body.Close()

	return nil
}
