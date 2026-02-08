package emby

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/celsian/iptv-manager/internal/config"
)

func TestNewClient(t *testing.T) {
	tmpDir := t.TempDir()
	cfgManager, _ := config.NewManager(filepath.Join(tmpDir, "config.json"))

	client := NewClient(cfgManager)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestRefreshGuideNoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgManager, _ := config.NewManager(filepath.Join(tmpDir, "config.json"))

	client := NewClient(cfgManager)
	err := client.RefreshGuide()

	if err == nil {
		t.Error("RefreshGuide should fail with no config")
	}
}

func TestRefreshGuide(t *testing.T) {
	scheduledTasks := []ScheduleTask{
		{ID: "task1", Key: "OtherTask"},
		{ID: "task2", Key: "RefreshGuide"},
	}

	taskTriggered := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/emby/ScheduledTasks" {
			json.NewEncoder(w).Encode(scheduledTasks)
			return
		}
		if r.URL.Path == "/emby/ScheduledTasks/Running/task2" && r.Method == "POST" {
			taskTriggered = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{
		Emby: config.EmbyConfig{
			APIAddress: server.URL,
			APIKey:     "testkey",
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	client := NewClient(cfgManager)

	err := client.RefreshGuide()
	if err != nil {
		t.Fatalf("RefreshGuide failed: %v", err)
	}

	if !taskTriggered {
		t.Error("RefreshGuide task should have been triggered")
	}
}

func TestRefreshGuideTaskNotFound(t *testing.T) {
	scheduledTasks := []ScheduleTask{
		{ID: "task1", Key: "OtherTask"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(scheduledTasks)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := config.Config{
		Emby: config.EmbyConfig{
			APIAddress: server.URL,
			APIKey:     "testkey",
		},
	}
	data, _ := json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)

	cfgManager, _ := config.NewManager(cfgPath)
	client := NewClient(cfgManager)

	err := client.RefreshGuide()
	if err == nil {
		t.Error("RefreshGuide should fail when task not found")
	}
}
