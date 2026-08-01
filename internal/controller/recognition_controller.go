package controller

import (
	"errors"
	"io"
	"log"
	"net/http"

	"organizing-app-backend/internal/ai"
	"organizing-app-backend/internal/service"
)

// RecognitionController turns a photo into a suggested category + item name
// via Gemini, for the "AI Recognition" add-item flow. A nil aiClient (no
// GEMINI_API_KEY configured) makes every request fail with 503, so the
// feature degrades cleanly instead of the server refusing to start.
type RecognitionController struct {
	aiClient        *ai.Client
	categoryService service.CategoryService
}

func NewRecognitionController(aiClient *ai.Client, categoryService service.CategoryService) *RecognitionController {
	return &RecognitionController{aiClient: aiClient, categoryService: categoryService}
}

// maxRecognitionBytes caps the image sent to Gemini. The client downsizes
// photos before calling this endpoint, so a well-behaved request is a few
// hundred KB; this just bounds a misbehaving or malicious one.
const maxRecognitionBytes = 8 << 20

// Recognize accepts a multipart "image" field and responds with the category
// (matching one of the caller's existing categories) and a suggested item
// name, for the client to prefill onto the Add Item form.
func (c *RecognitionController) Recognize(w http.ResponseWriter, r *http.Request) {
	if c.aiClient == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("AI recognition is not configured"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRecognitionBytes)
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
	contentType := http.DetectContentType(head[:n])
	if _, ok := imageExtensions[contentType]; !ok {
		writeError(w, http.StatusBadRequest, errors.New("unsupported image type"))
		return
	}

	rest, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	imageData := append(head[:n], rest...)

	categories, err := c.categoryService.ListCategories(CurrentUser(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	names := make([]string, 0, len(categories))
	for _, cat := range categories {
		names = append(names, cat.Name)
	}
	if len(names) == 0 {
		writeError(w, http.StatusUnprocessableEntity, errors.New("no categories to classify into — add a category first"))
		return
	}

	result, err := c.aiClient.Recognize(r.Context(), imageData, contentType, names)
	if err != nil {
		log.Printf("recognize item: %v", err)
		writeError(w, http.StatusBadGateway, errors.New("couldn't recognize this item"))
		return
	}

	writeJSON(w, http.StatusOK, result)
}
