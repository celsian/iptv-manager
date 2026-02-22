package main

import (
	"context"
	"embed"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/celsian/iptv-manager/internal/api"
	"github.com/celsian/iptv-manager/internal/autosearch"
	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/config"
	"github.com/celsian/iptv-manager/internal/emby"
	"github.com/celsian/iptv-manager/internal/iptv"
	"github.com/celsian/iptv-manager/internal/playlists"
)

//go:embed all:web/dist
var staticFS embed.FS

func main() {
	port := flag.String("port", "8080", "Server port")
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(dataDir, "config.json")
	}

	channelsPath := filepath.Join(dataDir, "channels.json")

	cfgManager, err := config.NewManager(cfgPath)
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	channelStore, err := channels.NewStore(channelsPath)
	if err != nil {
		log.Fatalf("Failed to initialize channel store: %v", err)
	}

	playlistManager := playlists.NewManager(cfgManager, channelStore, dataDir)
	playlistManager.StartScheduler()

	// Initialize auto search
	autoSearchPath := filepath.Join(dataDir, "autosearch.json")
	autoSearchStore, err := autosearch.NewStore(autoSearchPath)
	if err != nil {
		log.Fatalf("Failed to initialize auto search store: %v", err)
	}

	iptvProvider := iptv.NewProvider(cfgManager)
	embyClient := emby.NewClient(cfgManager)
	discordWebhook := cfgManager.Get().DiscordWebhook

	autoSearchExecutor := autosearch.NewExecutor(autoSearchStore, channelStore, iptvProvider, playlistManager, embyClient, discordWebhook)
	autoSearchScheduler := autosearch.NewScheduler(autoSearchExecutor, autoSearchStore)
	autoSearchScheduler.Start()

	server := api.NewServer(cfgManager, channelStore, playlistManager, staticFS)
	server.SetAutoSearch(autoSearchStore, autoSearchExecutor, autoSearchScheduler)

	addr := ":" + *port
	httpServer := &http.Server{
		Addr:    addr,
		Handler: server.Router(),
	}

	log.Printf("Starting IPTV Manager on http://localhost%s", addr)
	log.Printf("M3U playlist available at http://localhost%s/m3u/iptv-manager.m3u", addr)

	// Graceful shutdown on SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down...")

	autoSearchScheduler.Stop()
	playlistManager.StopScheduler()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
