package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/celsian/iptv-manager/internal/autosearch"
	"github.com/celsian/iptv-manager/internal/channels"
	"github.com/celsian/iptv-manager/internal/config"
	"github.com/celsian/iptv-manager/internal/emby"
	"github.com/celsian/iptv-manager/internal/iptv"
	"github.com/celsian/iptv-manager/internal/playlists"
)

type Server struct {
	cfg                 *config.Manager
	iptvProvider        iptv.Provider
	embyClient          *emby.Client
	channelStore        *channels.Store
	playlistManager     *playlists.Manager
	autoSearchStore     *autosearch.Store
	autoSearchExecutor  *autosearch.Executor
	autoSearchScheduler *autosearch.Scheduler
	staticFS            embed.FS
}

func NewServer(cfg *config.Manager, channelStore *channels.Store, playlistManager *playlists.Manager, staticFS embed.FS) *Server {
	return &Server{
		cfg:             cfg,
		iptvProvider:    iptv.NewProvider(cfg),
		embyClient:      emby.NewClient(cfg),
		channelStore:    channelStore,
		playlistManager: playlistManager,
		staticFS:        staticFS,
	}
}

func (s *Server) SetAutoSearch(store *autosearch.Store, executor *autosearch.Executor, scheduler *autosearch.Scheduler) {
	s.autoSearchStore = store
	s.autoSearchExecutor = executor
	s.autoSearchScheduler = scheduler
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// IPTV search (still queries IPTV provider)
	mux.HandleFunc("GET /api/iptv/search", s.handleChannelSearch)
	mux.HandleFunc("POST /api/iptv/toggle", s.handleChannelToggle)
	mux.HandleFunc("GET /api/iptv/playlists", s.handleGetPlaylists)

	// Local channel management (replaces xTeVe)
	mux.HandleFunc("GET /api/channels", s.handleLocalChannels)
	mux.HandleFunc("GET /api/channels/enabled", s.handleLocalEnabled)
	mux.HandleFunc("GET /api/channels/{iptvId}", s.handleLocalChannelGet)
	mux.HandleFunc("POST /api/channels", s.handleLocalChannelSave)
	mux.HandleFunc("POST /api/channels/disable", s.handleLocalChannelDisable)
	mux.HandleFunc("GET /api/channels/nearby", s.handleLocalNearby)
	mux.HandleFunc("GET /api/channels/groups", s.handleLocalGroupTitles)
	mux.HandleFunc("GET /api/channels/next-number", s.handleNextChannelNumber)
	mux.HandleFunc("GET /api/channels/check-conflict", s.handleCheckChannelConflict)

	// Playlist management
	mux.HandleFunc("GET /api/playlists/sources", s.handleGetPlaylistSources)
	mux.HandleFunc("POST /api/playlists/dirty", s.handleMarkPlaylistDirty)
	mux.HandleFunc("GET /api/playlists/update-if-dirty", s.handleUpdatePlaylistIfDirty)
	mux.HandleFunc("POST /api/playlists/update", s.handleUpdatePlaylist)
	mux.HandleFunc("POST /api/playlists/update-all", s.handleUpdateAllPlaylists)
	mux.HandleFunc("GET /api/playlists/status", s.handlePlaylistStatus)
	mux.HandleFunc("GET /api/playlists/channel-url", s.handleGetChannelURL)
	mux.HandleFunc("GET /api/playlists/channels", s.handleGetPlaylistChannels)

	// M3U endpoint
	mux.HandleFunc("GET /m3u/iptv-manager.m3u", s.handleM3U)

	// Preview
	mux.HandleFunc("GET /api/preview/{channelId}", s.handlePreview)
	mux.HandleFunc("GET /api/preview/{channelId}/url", s.handlePreviewURL)

	// Settings
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)

	// Emby
	mux.HandleFunc("POST /api/emby/refresh", s.handleEmbyRefresh)

	// Discord
	mux.HandleFunc("POST /api/discord/test", s.handleTestDiscordWebhook)

	// Auto Search Jobs
	mux.HandleFunc("GET /api/autosearch/jobs", s.handleGetAutoSearchJobs)
	mux.HandleFunc("GET /api/autosearch/jobs/{id}", s.handleGetAutoSearchJob)
	mux.HandleFunc("POST /api/autosearch/jobs", s.handleCreateAutoSearchJob)
	mux.HandleFunc("PUT /api/autosearch/jobs/{id}", s.handleUpdateAutoSearchJob)
	mux.HandleFunc("DELETE /api/autosearch/jobs/{id}", s.handleDeleteAutoSearchJob)
	mux.HandleFunc("POST /api/autosearch/jobs/{id}/run", s.handleRunAutoSearchJob)
	mux.HandleFunc("POST /api/autosearch/preview", s.handlePreviewAutoSearchJob)

	// Serve static files
	staticContent, err := fs.Sub(s.staticFS, "web/dist")
	if err != nil {
		staticContent, _ = fs.Sub(s.staticFS, ".")
	}

	fileServer := http.FileServer(http.FS(staticContent))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		if _, err := fs.Stat(staticContent, path[1:]); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}))

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
