package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/service"
)

// LookController wires HTTP requests to the LookService.
type LookController struct {
	service service.LookService
}

func NewLookController(s service.LookService) *LookController {
	return &LookController{service: s}
}

func (c *LookController) List(w http.ResponseWriter, r *http.Request) {
	looks, err := c.service.List(r.Context(), CurrentUser(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, looks)
}

func (c *LookController) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	look, err := c.service.Get(r.Context(), CurrentUser(r.Context()).ID, id)
	if err != nil {
		writeLookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, look)
}

func (c *LookController) Create(w http.ResponseWriter, r *http.Request) {
	var look model.Look
	if err := json.NewDecoder(r.Body).Decode(&look); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	created, err := c.service.Create(r.Context(), CurrentUser(r.Context()).ID, look)
	if err != nil {
		writeLookError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (c *LookController) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var look model.Look
	if err := json.NewDecoder(r.Body).Decode(&look); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	updated, err := c.service.Update(r.Context(), CurrentUser(r.Context()).ID, id, look)
	if err != nil {
		writeLookError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (c *LookController) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := c.service.Delete(r.Context(), CurrentUser(r.Context()).ID, id); err != nil {
		writeLookError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeLookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrLookNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, service.ErrInvalidLook):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
