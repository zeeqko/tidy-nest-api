package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/service"
)

// InventoryController wires HTTP requests to the InventoryService.
type InventoryController struct {
	service service.InventoryService
}

func NewInventoryController(s service.InventoryService) *InventoryController {
	return &InventoryController{service: s}
}

func (c *InventoryController) List(w http.ResponseWriter, r *http.Request) {
	items, err := c.service.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (c *InventoryController) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	item, err := c.service.Get(id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (c *InventoryController) Create(w http.ResponseWriter, r *http.Request) {
	var item model.InventoryItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	created, err := c.service.Create(item)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (c *InventoryController) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var item model.InventoryItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	updated, err := c.service.Update(id, item)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (c *InventoryController) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := c.service.Delete(id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
