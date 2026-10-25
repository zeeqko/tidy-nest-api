package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

// writeJSONWithETag writes v as a 200 JSON response exactly like writeJSON
// (same encoder, so the body is byte-identical), but also sets a strong ETag
// derived from the encoded body and Cache-Control: private, no-cache. When the
// request's If-None-Match matches, it replies 304 with no body instead.
func writeJSONWithETag(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	body := buf.Bytes()

	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "private, no-cache")

	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h.Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

// etagMatches reports whether an If-None-Match header value matches etag.
// It accepts "*", comma-separated lists, and weak (W/) validators, since
// weak comparison is what If-None-Match requires.
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		candidate = strings.TrimPrefix(candidate, "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}
