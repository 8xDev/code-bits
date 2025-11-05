package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/posts-presign/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Post model
type Post struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	ObjectKey   string    `json:"object_key"`
	MediaURL    string    `json:"media_url"`
	MediaType   string    `json:"media_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Handler struct {
	db             *sql.DB
	minio          *storage.MinioClient
	apiKey         string
	maxUploadMB    int
	maxUploadBytes int64
}

func New(db *sql.DB, minio *storage.MinioClient, apiKey string, maxUploadMB int) *Handler {
	return &Handler{
		db:             db,
		minio:          minio,
		apiKey:         apiKey,
		maxUploadMB:    maxUploadMB,
		maxUploadBytes: int64(maxUploadMB) * 1024 * 1024,
	}
}

// AuthMiddleware enforces X-API-KEY on state-changing endpoints
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// allow read-only GET without API key
		if r.Method == http.MethodGet || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-API-KEY")
		if key == "" {
			// also allow query param for testing
			key = r.URL.Query().Get("api_key")
		}
		if key == "" || key != h.apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type InitUploadRequest struct {
	Filename string `json:"filename"`
	// prefer "post" for form POST policy or "put" for presigned PUT
	Method string `json:"method,omitempty"`
}

// InitUpload returns presigned POST form data (recommended) or presigned PUT URL.
func (h *Handler) InitUpload(w http.ResponseWriter, r *http.Request) {
	// parse JSON
	var req InitUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Filename) == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if method != "put" && method != "post" {
		method = "post"
	}
	ext := ""
	if idx := strings.LastIndex(req.Filename, "."); idx >= 0 {
		ext = req.Filename[idx:]
	}

	// generate unique key
	objKey := uuid.New().String() + ext

	// use SSE on upload (SSE-S3): server-side encryption at rest
	useSSE := true

	ctx := r.Context()
	if method == "put" {
		// presign PUT
		url, err := h.minio.GeneratePresignedPut(ctx, objKey, 15*time.Minute)
		if err != nil {
			log.Error().Err(err).Msg("presign put")
			http.Error(w, "failed to create presigned url", http.StatusInternalServerError)
			return
		}
		resp := map[string]interface{}{
			"method":    "put",
			"objectKey": objKey,
			"url":       url,
			"expires":   15 * 60, // seconds
		}
		jsonResponse(w, resp, http.StatusOK)
		return
	}

	// presigned POST (form) with content length policy
	expiry := time.Now().Add(15 * time.Minute)
	minBytes := int64(1)
	maxBytes := h.maxUploadBytes
	formFields, postURL, err := h.minio.GeneratePresignedPost(ctx, objKey, expiry, minBytes, maxBytes, "", useSSE)
	if err != nil {
		log.Error().Err(err).Msg("presign post")
		http.Error(w, "failed to create post policy", http.StatusInternalServerError)
		return
	}
	resp := map[string]interface{}{
		"method":     "post",
		"objectKey":  objKey,
		"url":        postURL,
		"fields":     formFields,
		"max_bytes":  maxBytes,
		"expires_at": expiry.Format(time.RFC3339),
	}
	jsonResponse(w, resp, http.StatusOK)
}

// CreatePostFromKey creates DB entry after client uploaded objectKey successfully.
type CreatePostRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ObjectKey   string `json:"object_key"`
}

func (h *Handler) CreatePostFromKey(w http.ResponseWriter, r *http.Request) {
	var req CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.ObjectKey) == "" {
		http.Error(w, "title and object_key required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	// Verify object exists and get metadata
	objInfo, err := h.minio.StatObject(ctx, req.ObjectKey)
	if err != nil {
		log.Error().Err(err).Msg("stat object")
		http.Error(w, "object not found or not accessible", http.StatusBadRequest)
		return
	}
	mediaType := "image"
	if strings.HasPrefix(objInfo.ContentType, "video/") {
		mediaType = "video"
	}
	mediaURL := h.minio.ConstructPublicURL(req.ObjectKey)
	// Insert DB
	var id int64
	err = h.db.QueryRowContext(ctx,
		"INSERT INTO posts (title, description, object_key, media_url, media_type, size_bytes) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id",
		req.Title, req.Description, req.ObjectKey, mediaURL, mediaType, objInfo.Size).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("db insert")
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	p := Post{
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
		ObjectKey:   req.ObjectKey,
		MediaURL:    mediaURL,
		MediaType:   mediaType,
		SizeBytes:   objInfo.Size,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	jsonResponse(w, p, http.StatusCreated)
}

func (h *Handler) ListPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), "SELECT id,title,description,object_key,media_url,media_type,size_bytes,created_at,updated_at FROM posts ORDER BY created_at DESC")
	if err != nil {
		log.Error().Err(err).Msg("list posts")
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.ObjectKey, &p.MediaURL, &p.MediaType, &p.SizeBytes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			log.Error().Err(err).Msg("scan")
			http.Error(w, "db scan error", http.StatusInternalServerError)
			return
		}
		out = append(out, p)
	}
	jsonResponse(w, out, http.StatusOK)
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var p Post
	err = h.db.QueryRowContext(r.Context(), "SELECT id,title,description,object_key,media_url,media_type,size_bytes,created_at,updated_at FROM posts WHERE id=$1", id).
		Scan(&p.ID, &p.Title, &p.Description, &p.ObjectKey, &p.MediaURL, &p.MediaType, &p.SizeBytes, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("get post")
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, p, http.StatusOK)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/posts/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	// get object key
	var objectKey string
	err = h.db.QueryRowContext(r.Context(), "SELECT object_key FROM posts WHERE id=$1", id).Scan(&objectKey)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	// delete from minio (best-effort)
	_ = h.minio.Delete(r.Context(), objectKey)
	// delete DB row
	_, err = h.db.ExecContext(r.Context(), "DELETE FROM posts WHERE id=$1", id)
	if err != nil {
		http.Error(w, "db delete error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Helper utilities
func jsonResponse(w http.ResponseWriter, v interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
