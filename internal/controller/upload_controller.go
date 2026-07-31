package controller

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"organizing-app-backend/internal/storage"
)

// UploadController stores user-submitted item photos in the configured object
// store (R2 in deployed environments) and streams them back out. Files get
// random names, so URLs are unguessable and never collide.
type UploadController struct {
	store storage.Store
}

func NewUploadController(store storage.Store) *UploadController {
	return &UploadController{store: store}
}

const maxUploadBytes = 15 << 20 // phone camera photos, with headroom

// Only types http.DetectContentType can sniff; the client converts anything
// else (e.g. HEIC) to JPEG before uploading.
var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// Upload accepts a multipart "image" field and responds with the URL the
// file will be served from, for the client to store on the item.
func (c *UploadController) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("missing image file"))
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	contentType := http.DetectContentType(head[:n])
	ext, ok := imageExtensions[contentType]
	if !ok {
		writeError(w, http.StatusBadRequest, errors.New("unsupported image type"))
		return
	}
	// Rewind past the sniffed header so the whole file gets stored.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	nameBytes := make([]byte, 16)
	if _, err := rand.Read(nameBytes); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	key := hex.EncodeToString(nameBytes) + ext

	// The multipart layer knows the exact size, which R2 needs up front: its
	// S3 endpoint does not accept the SDK's chunked streaming uploads.
	if err := c.store.Put(r.Context(), key, contentType, file, header.Size); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"url": "/uploads/" + key})
}

// Serve streams a previously uploaded image back to the browser. Photos stay
// private: they are fetched from the bucket with the backend's credentials and
// only handed to a caller that passed the session check on this route.
func (c *UploadController) Serve(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}

	object, err := c.store.Get(r.Context(), name)
	if errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer object.Body.Close()

	w.Header().Set("Content-Type", object.ContentType)
	if object.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	}
	// Filenames are random and never reused, so the content is immutable.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	io.Copy(w, object.Body)
}
