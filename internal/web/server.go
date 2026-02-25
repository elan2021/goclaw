package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"goclaw/internal/gateway"
	"goclaw/internal/scheduler"
)

//go:embed all:dist
var staticDist embed.FS

type Server struct {
	addr    string
	gw      *gateway.Gateway
	sched   *scheduler.Scheduler
	version string
}

func New(port int, gw *gateway.Gateway, sched *scheduler.Scheduler) *Server {
	return &Server{
		addr:    fmt.Sprintf(":%d", port),
		gw:      gw,
		sched:   sched,
		version: "0.2.0-phase4",
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("POST /api/sessions/{id}/kill", s.handleKillSession)
	mux.HandleFunc("GET /api/schedules", s.handleSchedules)

	// Static Files (Frontend)
	distFS, err := fs.Sub(staticDist, "dist")
	if err != nil {
		slog.Warn("static frontend not found, API only mode", "error", err)
	} else {
		mux.Handle("/", http.FileServer(http.FS(distFS)))
	}

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("web dashboard listening", "addr", s.addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

type statusResp struct {
	Version  string `json:"version"`
	Uptime   string `json:"uptime"`
	Sessions int    `json:"active_sessions"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := statusResp{
		Version:  s.version,
		Uptime:   time.Since(startTime).String(),
		Sessions: s.gw.ActiveSessions(),
	}
	s.json(w, resp)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	ids := s.gw.SessionIDs()
	s.json(w, map[string]interface{}{"sessions": ids})
}

func (s *Server) handleKillSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	s.gw.KillSession(id)
	s.json(w, map[string]string{"status": "killed", "id": id})
}

func (s *Server) handleSchedules(w http.ResponseWriter, r *http.Request) {
	// Note: We need a way to list tasks from scheduler.
	// For now, returning an empty list or placeholders.
	s.json(w, map[string]interface{}{"schedules": []string{}})
}

func (s *Server) json(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

var startTime = time.Now()
