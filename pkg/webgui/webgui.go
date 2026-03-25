package webgui

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/u16-io/FindSenryu4Discord/config"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
)

// Server represents the WebGUI HTTP server.
type Server struct {
	server *http.Server
}

// NewServer creates a new WebGUI server.
func NewServer(port int) *Server {
	mux := http.NewServeMux()

	s := &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
	}

	// Pages
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/upload", s.handleUploadPage)

	// API
	mux.HandleFunc("/api/senryu", s.handleSenryuList)
	mux.HandleFunc("/api/senryu/", s.handleSenryuImage)
	mux.HandleFunc("/api/background", s.handleBackgroundUpload)
	mux.HandleFunc("/api/background/", s.handleBackgroundGet)

	return s
}

// Start starts the WebGUI server in background.
func (s *Server) Start() error {
	logger.Info("Starting WebGUI server", "addr", s.server.Addr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("WebGUI server error", "error", err)
		}
	}()
	return nil
}

// Stop stops the WebGUI server.
func (s *Server) Stop(ctx context.Context) error {
	logger.Info("Stopping WebGUI server")
	return s.server.Shutdown(ctx)
}

// StartServer creates and starts the WebGUI server if enabled.
func StartServer() (*Server, error) {
	conf := config.GetConf()
	if !conf.Web.Enabled {
		logger.Info("WebGUI server is disabled")
		return nil, nil
	}

	s := NewServer(conf.Web.Port)
	if err := s.Start(); err != nil {
		return nil, err
	}
	return s, nil
}
