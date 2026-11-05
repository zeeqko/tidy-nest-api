package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/service"
)

// fakeCategoryService records UpdateCategory calls; every other method panics
// via the embedded nil interface, which these tests never reach.
type fakeCategoryService struct {
	service.CategoryService
	called bool
	group  *string
}

func (f *fakeCategoryService) UpdateCategory(_ context.Context, _, id int64, name, _, _ string, group *string) (model.Category, error) {
	f.called = true
	f.group = group
	return model.Category{ID: id, Name: name, StylebookGroup: group}, nil
}

func putCategory(t *testing.T, fake *fakeCategoryService, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Put("/categories/{id}", NewCategoryController(fake).UpdateCategory)
	req := httptest.NewRequest(http.MethodPut, "/categories/7", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestUpdateCategoryInvalidGroupReturns400(t *testing.T) {
	fake := &fakeCategoryService{}
	rec := putCategory(t, fake, `{"name":"X","stylebookGroup":"hats"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if fake.called {
		t.Fatal("service must not be called for an invalid group")
	}
}

func TestUpdateCategoryOmittedGroupPassesNil(t *testing.T) {
	fake := &fakeCategoryService{}
	rec := putCategory(t, fake, `{"name":"X"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fake.group != nil {
		t.Fatalf("group = %q, want nil", *fake.group)
	}
}

func TestUpdateCategoryEmptyGroupPassesEmptyPointer(t *testing.T) {
	fake := &fakeCategoryService{}
	rec := putCategory(t, fake, `{"name":"X","stylebookGroup":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fake.group == nil || *fake.group != "" {
		t.Fatalf("group = %v, want pointer to empty string", fake.group)
	}
}

func TestUpdateCategoryValidGroupPassesValue(t *testing.T) {
	fake := &fakeCategoryService{}
	rec := putCategory(t, fake, `{"name":"X","stylebookGroup":"shoes"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fake.group == nil || *fake.group != "shoes" {
		t.Fatalf("group = %v, want shoes", fake.group)
	}
	if !strings.Contains(rec.Body.String(), `"stylebookGroup":"shoes"`) {
		t.Fatalf("body missing stylebookGroup: %s", rec.Body.String())
	}
}
