package webgui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chai2010/webp"
	"github.com/u16-io/FindSenryu4Discord/pkg/logger"
	"github.com/u16-io/FindSenryu4Discord/pkg/senryuimg"
	"github.com/u16-io/FindSenryu4Discord/service"
)

const (
	backgroundDir = "data/backgrounds"
	maxUploadSize = 10 << 20 // 10MB
)

// handleIndex serves the senryu list page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

// handleUploadPage serves the background upload page.
func (s *Server) handleUploadPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(uploadHTML))
}

// handleSenryuList returns paginated senryu list as JSON.
func (s *Server) handleSenryuList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serverID := r.URL.Query().Get("guild_id")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	senryus, total, err := service.GetSenryuList(serverID, page, pageSize)
	if err != nil {
		logger.Error("Failed to get senryu list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	type senryuJSON struct {
		ID        int    `json:"id"`
		ServerID  string `json:"server_id"`
		AuthorID  string `json:"author_id"`
		Kamigo    string `json:"kamigo"`
		Nakasichi string `json:"nakasichi"`
		Simogo    string `json:"simogo"`
		Spoiler   bool   `json:"spoiler"`
		CreatedAt string `json:"created_at"`
		ImageURL  string `json:"image_url"`
	}

	var items []senryuJSON
	for _, s := range senryus {
		spoiler := false
		if s.Spoiler != nil {
			spoiler = *s.Spoiler
		}
		items = append(items, senryuJSON{
			ID:        s.ID,
			ServerID:  s.ServerID,
			AuthorID:  s.AuthorID,
			Kamigo:    s.Kamigo,
			Nakasichi: s.Nakasichi,
			Simogo:    s.Simogo,
			Spoiler:   spoiler,
			CreatedAt: s.CreatedAt.Format("2006-01-02 15:04:05"),
			ImageURL:  fmt.Sprintf("/api/senryu/%d/image", s.ID),
		})
	}

	resp := map[string]interface{}{
		"senryus":   items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSenryuImage generates and returns a senryu image as webp.
// URL pattern: /api/senryu/{id}/image
func (s *Server) handleSenryuImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse ID from URL: /api/senryu/123/image
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/senryu/"), "/")
	if len(parts) < 2 || parts[1] != "image" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	senryu, err := service.GetSenryuByIDGlobal(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Load background if available
	var bgData []byte
	bg, err := service.GetBackground(senryu.ServerID)
	if err == nil && bg != nil {
		if data, err := os.ReadFile(bg.FilePath); err == nil {
			bgData = data
		}
	}

	opts := senryuimg.RenderOptions{
		Kamigo:     senryu.Kamigo,
		Nakasichi:  senryu.Nakasichi,
		Simogo:     senryu.Simogo,
		AuthorName: senryu.AuthorID, // Web API doesn't have Discord session, use ID
		Background: bgData,
	}

	imgData, err := senryuimg.RenderSenryu(opts)
	if err != nil {
		logger.Error("Failed to render senryu image", "error", err)
		http.Error(w, "Failed to generate image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"senryu_%d.webp\"", id))
	w.Write(imgData)
}

// handleBackgroundUpload handles background image upload.
func (s *Server) handleBackgroundUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large (max 10MB)", http.StatusBadRequest)
		return
	}

	guildID := r.FormValue("guild_id")
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	imgData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Decode the image
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		http.Error(w, "Invalid image file. Supported formats: JPEG, PNG, GIF, WebP", http.StatusBadRequest)
		return
	}

	// Convert to webp
	var webpBuf bytes.Buffer
	if err := webp.Encode(&webpBuf, img, &webp.Options{Quality: 90}); err != nil {
		logger.Error("Failed to encode background to webp", "error", err)
		http.Error(w, "Failed to convert image", http.StatusInternalServerError)
		return
	}

	// Ensure directory exists
	if err := os.MkdirAll(backgroundDir, 0755); err != nil {
		logger.Error("Failed to create backgrounds directory", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Save file
	filePath := filepath.Join(backgroundDir, guildID+".webp")
	if err := os.WriteFile(filePath, webpBuf.Bytes(), 0644); err != nil {
		logger.Error("Failed to save background image", "error", err)
		http.Error(w, "Failed to save image", http.StatusInternalServerError)
		return
	}

	// Update DB
	if err := service.UpsertBackground(guildID, filePath); err != nil {
		logger.Error("Failed to save background record", "error", err)
		http.Error(w, "Failed to save record", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"guild_id": guildID,
		"message":  "Background image uploaded and converted to webp",
	})
}

// handleBackgroundGet returns the background image for a guild.
// URL pattern: /api/background/{guild_id}
func (s *Server) handleBackgroundGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	guildID := strings.TrimPrefix(r.URL.Path, "/api/background/")
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}

	bg, err := service.GetBackground(guildID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	data, err := os.ReadFile(bg.FilePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Write(data)
}
