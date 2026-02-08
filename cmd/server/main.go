package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/celsian/iptv-manager/internal/api"
	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/config"
	"github.com/celsian/iptv-manager/internal/playlists"
)

//go:embed all:web/dist
var staticFS embed.FS

func main() {
	port := flag.String("port", "8080", "Server port")
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// Determine data directory - defaults to "./data" for local development
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Config path
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(dataDir, "config.json")
	}

	// Channels data path
	channelsPath := filepath.Join(dataDir, "channels.json")

	// Initialize config manager
	cfgManager, err := config.NewManager(cfgPath)
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	// Initialize channel store
	channelStore, err := channels.NewStore(channelsPath)
	if err != nil {
		log.Fatalf("Failed to initialize channel store: %v", err)
	}

	// Initialize playlist manager
	playlistManager := playlists.NewManager(cfgManager, channelStore, dataDir)
	playlistManager.StartScheduler()

	// Create and start server
	server := api.NewServer(cfgManager, channelStore, playlistManager, staticFS)

	addr := ":" + *port
	log.Printf("Starting IPTV Manager on http://localhost%s", addr)
	log.Printf("M3U playlist available at http://localhost%s/m3u/iptv-manager.m3u", addr)

	if err := http.ListenAndServe(addr, server.Router()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
