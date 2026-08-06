package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"organizing-app-backend/internal/service"
)

// CategoryController wires HTTP requests to the CategoryService.
type CategoryController struct {
	service service.CategoryService
}

func NewCategoryController(s service.CategoryService) *CategoryController {
	return &CategoryController{service: s}
}

type namePayload struct {
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Colour string `json:"colour"`
}

func (c *CategoryController) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := c.service.ListCategories(r.Context(), CurrentUser(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, categories)
}

func (c *CategoryController) CreateCategory(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeNamed(w, r)
	if !ok {
		return
	}
	category, err := c.service.CreateCategory(r.Context(), CurrentUser(r.Context()).ID, payload.Name, payload.Icon, payload.Colour)
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, category)
}

func (c *CategoryController) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeNamed(w, r)
	if !ok {
		return
	}
	category, err := c.service.UpdateCategory(r.Context(), CurrentUser(r.Context()).ID, id, payload.Name, payload.Icon, payload.Colour)
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, category)
}

func (c *CategoryController) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	if err := c.service.DeleteCategory(r.Context(), CurrentUser(r.Context()).ID, id); err != nil {
		writeCategoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *CategoryController) CreateSubCategory(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := parsePathID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeNamed(w, r)
	if !ok {
		return
	}
	subCategory, err := c.service.CreateSubCategory(r.Context(), CurrentUser(r.Context()).ID, categoryID, payload.Name)
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, subCategory)
}

func (c *CategoryController) DeleteSubCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	if err := c.service.DeleteSubCategory(r.Context(), CurrentUser(r.Context()).ID, id); err != nil {
		writeCategoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *CategoryController) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := c.service.ListTags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

func (c *CategoryController) CreateTag(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeNamed(w, r)
	if !ok {
		return
	}
	tag, err := c.service.CreateTag(r.Context(), payload.Name, payload.Colour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

func (c *CategoryController) AttachTag(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := parsePathID(w, r)
	if !ok {
		return
	}
	payload, ok := decodeNamed(w, r)
	if !ok {
		return
	}
	tag, err := c.service.AttachTag(r.Context(), categoryID, payload.Name, payload.Colour)
	if err != nil {
		writeCategoryError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

func (c *CategoryController) DetachTag(w http.ResponseWriter, r *http.Request) {
	categoryID, ok := parsePathID(w, r)
	if !ok {
		return
	}
	tagID, err := strconv.ParseInt(chi.URLParam(r, "tagId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, service.ErrNotFound)
		return
	}
	if err := c.service.DetachTag(r.Context(), CurrentUser(r.Context()).ID, categoryID, tagID); err != nil {
		writeCategoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *CategoryController) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathID(w, r)
	if !ok {
		return
	}
	if err := c.service.DeleteTag(r.Context(), id); err != nil {
		writeCategoryError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeNamed(w http.ResponseWriter, r *http.Request) (namePayload, bool) {
	var payload namePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return namePayload{}, false
	}
	if payload.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name is required"))
		return namePayload{}, false
	}
	return payload, true
}

func parsePathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, service.ErrNotFound)
		return 0, false
	}
	return id, true
}

func writeCategoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, service.ErrDuplicateName):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
