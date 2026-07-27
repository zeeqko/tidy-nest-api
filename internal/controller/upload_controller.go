package controller

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// UploadController stores user-submitted item photos on disk and serves them
// back. Files get random names, so URLs are unguessable and never collide.
type UploadController struct {
	dir string
}

func NewUploadController(dir string) *UploadController {
	return &UploadController{dir: dir}
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

	file, _, err := r.FormFile("image")
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
	ext, ok := imageExtensions[http.DetectContentType(head[:n])]
	if !ok {
		writeError(w, http.StatusBadRequest, errors.New("unsupported image type"))
		return
	}

	nameBytes := make([]byte, 16)
	if _, err := rand.Read(nameBytes); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	name := hex.EncodeToString(nameBytes) + ext

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	dst, err := os.Create(filepath.Join(c.dir, name))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer dst.Close()

	if _, err := dst.Write(head[:n]); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"url": "/uploads/" + name})
}

// Serve returns a previously uploaded image by filename.
func (c *UploadController) Serve(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	// Filenames are random and never reused, so the content is immutable.
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(c.dir, name))
}
