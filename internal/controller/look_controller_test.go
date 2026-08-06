package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"organizing-app-backend/internal/db"
	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/service"
)

// testEmailCounter guarantees unique emails across tests run in the same
// process, since email uniqueness is enforced at the DB level.
var testEmailCounter int64

// testDB opens a connection to the Postgres instance used for local
// development (same DATABASE_URL convention as cmd/server/main.go) and
// applies any pending migrations. Tests skip (rather than fail) when no
// database is reachable, so `go build`/`go vet` and unrelated `go test`
// runs never require Postgres to be up.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/organizing_app?sslmode=disable"
	}

	ctx := context.Background()
	database, err := db.Open(ctx, dsn)
	if err != nil {
		t.Skipf("postgres not available at %q: %v", dsn, err)
	}
	if err := db.Migrate(ctx, database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// newTestUser signs up a fresh user and returns its id plus a session
// cookie usable on requests to a RequireAuth-guarded route. The user (and,
// via ON DELETE CASCADE, everything it owns: Inventories, Looks, LookItems)
// is removed when the test finishes.
func newTestUser(t *testing.T, database *pgxpool.Pool) (int64, *http.Cookie) {
	t.Helper()

	ctx := context.Background()
	authService := service.NewAuthService(database)
	n := atomic.AddInt64(&testEmailCounter, 1)
	email := fmt.Sprintf("look-test-%d-%d@example.com", os.Getpid(), n)

	user, err := authService.Signup(ctx, "Look Test User", email, "password123")
	if err != nil {
		t.Fatalf("signup: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(ctx, `DELETE FROM Users WHERE id = $1`, user.ID); err != nil {
			t.Logf("cleanup user %d: %v", user.ID, err)
		}
	})

	token, expires, err := authService.CreateSession(ctx, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return user.ID, &http.Cookie{Name: sessionCookie, Value: token, Expires: expires}
}

// newTestItem inserts a minimal Inventories row directly (bypassing
// InventoryService, which needs a photo store this package doesn't set up)
// so look tests have a real, user-owned item id to reference.
func newTestItem(t *testing.T, database *pgxpool.Pool, userID int64, name string) int64 {
	t.Helper()
	return newTestItemWithPhotos(t, database, userID, name, "", "")
}

// newTestItemWithPhotos is newTestItem plus the ability to set imageURL/
// cutoutURL directly on the inserted row, so tests can exercise the
// Inventories -> LookItems join for those response-only fields without
// needing InventoryService's photo store.
func newTestItemWithPhotos(t *testing.T, database *pgxpool.Pool, userID int64, name, imageURL, cutoutURL string) int64 {
	t.Helper()

	var id int64
	err := database.QueryRow(
		context.Background(),
		`INSERT INTO Inventories (userId, name, quantity, imageURL, cutoutURL, createdAt, updatedAt)
		 VALUES ($1, $2, 1, NULLIF($3, ''), NULLIF($4, ''), now(), now()) RETURNING id`,
		userID, name, imageURL, cutoutURL,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert test item: %v", err)
	}
	return id
}

// newLookRouter wires the same /api/looks routes as router.New, minus the
// pieces (photo storage, AI client) unrelated to the Look domain.
func newLookRouter(database *pgxpool.Pool) http.Handler {
	authController := NewAuthController(service.NewAuthService(database))
	lookController := NewLookController(service.NewLookService(database))

	r := chi.NewRouter()
	r.Route("/api/looks", func(r chi.Router) {
		r.Use(authController.RequireAuth)
		r.Get("/", lookController.List)
		r.Post("/", lookController.Create)
		r.Get("/{id}", lookController.Get)
		r.Put("/{id}", lookController.Update)
		r.Delete("/{id}", lookController.Delete)
	})
	return r
}

func doJSON(t *testing.T, handler http.Handler, method, path string, cookie *http.Cookie, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodeLook(t *testing.T, rec *httptest.ResponseRecorder) model.Look {
	t.Helper()
	var look model.Look
	if err := json.Unmarshal(rec.Body.Bytes(), &look); err != nil {
		t.Fatalf("decode look response %q: %v", rec.Body.String(), err)
	}
	return look
}

func TestLookCreateListGetDelete(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	userID, cookie := newTestUser(t, database)
	itemID := newTestItem(t, database, userID, "Blue Denim Jacket")

	payload := model.Look{
		Name:     "Casual Friday",
		Occasion: "Casual",
		Items: []model.LookItem{
			{ItemID: strconv.FormatInt(itemID, 10), X: 1, Y: 2, Width: 100, Height: 200, Rotation: 15, ZIndex: 0},
		},
	}

	// Create.
	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeLook(t, rec)
	if created.ID == "" {
		t.Fatal("create: expected a non-empty id")
	}
	if created.Name != "Casual Friday" || created.Occasion != "Casual" {
		t.Fatalf("create: unexpected name/occasion: %+v", created)
	}
	if len(created.Items) != 1 {
		t.Fatalf("create: expected 1 item, got %d", len(created.Items))
	}
	got := created.Items[0]
	if got.ItemID != strconv.FormatInt(itemID, 10) {
		t.Fatalf("create: expected itemId %d, got %s", itemID, got.ItemID)
	}
	if got.Name != "Blue Denim Jacket" {
		t.Fatalf("create: expected joined name %q, got %q", "Blue Denim Jacket", got.Name)
	}
	if got.ImageURL != "" {
		t.Fatalf("create: expected empty imageURL (no photo on test item), got %q", got.ImageURL)
	}
	if got.X != 1 || got.Y != 2 || got.Width != 100 || got.Height != 200 || got.Rotation != 15 || got.ZIndex != 0 {
		t.Fatalf("create: transform not round-tripped: %+v", got)
	}

	// List.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var looks []model.Look
	if err := json.Unmarshal(rec.Body.Bytes(), &looks); err != nil {
		t.Fatalf("list: decode: %v", err)
	}
	if len(looks) != 1 || looks[0].ID != created.ID {
		t.Fatalf("list: expected exactly the created look, got %+v", looks)
	}

	// Get.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+created.ID, cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	fetched := decodeLook(t, rec)
	if fetched.ID != created.ID || len(fetched.Items) != 1 {
		t.Fatalf("get: unexpected look: %+v", fetched)
	}

	// Delete.
	rec = doJSON(t, handler, http.MethodDelete, "/api/looks/"+created.ID, cookie, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Get again -> 404.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+created.ID, cookie, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// rawLookItems decodes a look response body into generic JSON, returning the
// "items" array as raw maps so a test can assert a key (e.g. "cutoutURL") is
// entirely absent rather than merely empty — decoding into model.Look can't
// distinguish "" from a missing key.
func rawLookItems(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()

	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw look response %q: %v", rec.Body.String(), err)
	}
	return raw.Items
}

func TestLookItemCutoutURL(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	userID, cookie := newTestUser(t, database)
	plainItem := newTestItem(t, database, userID, "No Photo Jacket")
	cutoutItem := newTestItemWithPhotos(t, database, userID, "Cutout Jacket", "https://example.com/photo.jpg", "https://example.com/cutout.png")

	payload := model.Look{
		Name: "Cutout Test Look",
		Items: []model.LookItem{
			{ItemID: strconv.FormatInt(plainItem, 10), Width: 10, Height: 10, ZIndex: 0},
			// Client-sent cutoutURL must be ignored on write: the item's
			// actual Inventories row has no cutoutURL, so this must not
			// leak into the response.
			{ItemID: strconv.FormatInt(cutoutItem, 10), Width: 10, Height: 10, ZIndex: 1, CutoutURL: "https://evil.example/hack.png"},
		},
	}

	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookie, payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeLook(t, rec)
	if len(created.Items) != 2 {
		t.Fatalf("create: expected 2 items, got %d", len(created.Items))
	}

	var plainOut, cutoutOut model.LookItem
	for _, item := range created.Items {
		switch item.ItemID {
		case strconv.FormatInt(plainItem, 10):
			plainOut = item
		case strconv.FormatInt(cutoutItem, 10):
			cutoutOut = item
		}
	}
	if plainOut.CutoutURL != "" {
		t.Fatalf("create: expected empty cutoutURL for item with no photo, got %q", plainOut.CutoutURL)
	}
	if cutoutOut.CutoutURL != "https://example.com/cutout.png" {
		t.Fatalf("create: expected joined cutoutURL from Inventories, got %q (client-sent value must be ignored on write)", cutoutOut.CutoutURL)
	}

	// The JSON key itself must be entirely absent (omitempty), not present
	// as "cutoutURL":"", for the item with no cutout.
	rawItems := rawLookItems(t, rec)
	for _, raw := range rawItems {
		if raw["itemId"] == strconv.FormatInt(plainItem, 10) {
			if _, present := raw["cutoutURL"]; present {
				t.Fatalf("create: expected cutoutURL key to be omitted for item with no cutout, got %+v", raw)
			}
		}
		if raw["itemId"] == strconv.FormatInt(cutoutItem, 10) {
			if raw["cutoutURL"] != "https://example.com/cutout.png" {
				t.Fatalf("create: expected cutoutURL key present with joined value, got %+v", raw)
			}
		}
	}

	// List surfaces the same joined values.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/", cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var looks []model.Look
	if err := json.Unmarshal(rec.Body.Bytes(), &looks); err != nil {
		t.Fatalf("list: decode: %v", err)
	}
	if len(looks) != 1 || len(looks[0].Items) != 2 {
		t.Fatalf("list: expected 1 look with 2 items, got %+v", looks)
	}
	for _, item := range looks[0].Items {
		if item.ItemID == strconv.FormatInt(cutoutItem, 10) && item.CutoutURL != "https://example.com/cutout.png" {
			t.Fatalf("list: expected joined cutoutURL, got %q", item.CutoutURL)
		}
		if item.ItemID == strconv.FormatInt(plainItem, 10) && item.CutoutURL != "" {
			t.Fatalf("list: expected empty cutoutURL, got %q", item.CutoutURL)
		}
	}

	// Get-one surfaces the same joined values.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+created.ID, cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	fetched := decodeLook(t, rec)
	for _, item := range fetched.Items {
		if item.ItemID == strconv.FormatInt(cutoutItem, 10) && item.CutoutURL != "https://example.com/cutout.png" {
			t.Fatalf("get: expected joined cutoutURL, got %q", item.CutoutURL)
		}
		if item.ItemID == strconv.FormatInt(plainItem, 10) && item.CutoutURL != "" {
			t.Fatalf("get: expected empty cutoutURL, got %q", item.CutoutURL)
		}
	}
	rawFetchedItems := rawLookItems(t, rec)
	for _, raw := range rawFetchedItems {
		if raw["itemId"] == strconv.FormatInt(plainItem, 10) {
			if _, present := raw["cutoutURL"]; present {
				t.Fatalf("get: expected cutoutURL key to be omitted for item with no cutout, got %+v", raw)
			}
		}
	}
}

func TestLookListScopedToCaller(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	_, cookieA := newTestUser(t, database)
	userB, cookieB := newTestUser(t, database)
	itemB := newTestItem(t, database, userB, "Red Scarf")

	// User B creates a look.
	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookieB, model.Look{
		Name:  "B's Look",
		Items: []model.LookItem{{ItemID: strconv.FormatInt(itemB, 10), Width: 10, Height: 10}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create for user B: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// User A's list is empty.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/", cookieA, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list for user A: expected 200, got %d", rec.Code)
	}
	var looksA []model.Look
	if err := json.Unmarshal(rec.Body.Bytes(), &looksA); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(looksA) != 0 {
		t.Fatalf("list for user A: expected no looks, got %+v", looksA)
	}
}

func TestLookGetOnAnotherUsersLook404s(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	_, cookieA := newTestUser(t, database)
	userB, cookieB := newTestUser(t, database)
	itemB := newTestItem(t, database, userB, "Green Hat")

	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookieB, model.Look{
		Name:  "B's Look",
		Items: []model.LookItem{{ItemID: strconv.FormatInt(itemB, 10), Width: 10, Height: 10}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create for user B: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeLook(t, rec)

	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+created.ID, cookieA, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get another user's look: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLookCreateValidation(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	userA, cookieA := newTestUser(t, database)
	itemA := newTestItem(t, database, userA, "Own Shoes")
	userB, _ := newTestUser(t, database)
	itemB := newTestItem(t, database, userB, "Someone Else's Bag")

	cases := []struct {
		name string
		body model.Look
	}{
		{
			name: "blank name",
			body: model.Look{
				Name:  "   ",
				Items: []model.LookItem{{ItemID: strconv.FormatInt(itemA, 10), Width: 10, Height: 10}},
			},
		},
		{
			name: "empty items",
			body: model.Look{
				Name:  "No Pieces",
				Items: []model.LookItem{},
			},
		},
		{
			name: "foreign itemId",
			body: model.Look{
				Name:  "Borrowed Look",
				Items: []model.LookItem{{ItemID: strconv.FormatInt(itemB, 10), Width: 10, Height: 10}},
			},
		},
		{
			name: "unknown itemId",
			body: model.Look{
				Name:  "Made Up Look",
				Items: []model.LookItem{{ItemID: "99999999999", Width: 10, Height: 10}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookieA, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLookEndpointsRequireAuth(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	requests := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/looks/", nil},
		{http.MethodPost, "/api/looks/", model.Look{Name: "x", Items: []model.LookItem{{ItemID: "1"}}}},
		{http.MethodGet, "/api/looks/1", nil},
		{http.MethodPut, "/api/looks/1", model.Look{Name: "x", Items: []model.LookItem{{ItemID: "1"}}}},
		{http.MethodDelete, "/api/looks/1", nil},
	}

	for _, req := range requests {
		t.Run(req.method+" "+req.path, func(t *testing.T) {
			rec := doJSON(t, handler, req.method, req.path, nil, req.body)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestLookUpdateFullReplace exercises the full-replace contract: name,
// occasion, and the entire item set (removals, additions, re-ordered
// zIndex, changed transforms) all reflect the PUT body, updatedAt advances,
// and createdAt is untouched.
func TestLookUpdateFullReplace(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	userID, cookie := newTestUser(t, database)
	keptItem := newTestItem(t, database, userID, "Kept Sweater")
	removedItem := newTestItem(t, database, userID, "Removed Scarf")
	addedItem := newTestItem(t, database, userID, "Added Boots")

	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookie, model.Look{
		Name:     "Original Name",
		Occasion: "Casual",
		Items: []model.LookItem{
			{ItemID: strconv.FormatInt(keptItem, 10), X: 1, Y: 1, Width: 10, Height: 10, Rotation: 0, ZIndex: 0},
			{ItemID: strconv.FormatInt(removedItem, 10), X: 2, Y: 2, Width: 20, Height: 20, Rotation: 0, ZIndex: 1},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeLook(t, rec)

	// Ensure the update's updatedAt timestamp is measurably later than
	// createdAt/the initial updatedAt (now() resolves to microseconds, but
	// give it a comfortable margin rather than relying on bare round-trip
	// latency).
	time.Sleep(5 * time.Millisecond)

	updatePayload := model.Look{
		Name:     "Updated Name",
		Occasion: "Formal",
		Items: []model.LookItem{
			// Kept item, transform changed and re-ordered to the back.
			{ItemID: strconv.FormatInt(keptItem, 10), X: 99, Y: 88, Width: 50, Height: 60, Rotation: 45, ZIndex: 1},
			// New item, placed in front.
			{ItemID: strconv.FormatInt(addedItem, 10), X: 5, Y: 5, Width: 30, Height: 30, Rotation: 10, ZIndex: 0},
		},
	}
	rec = doJSON(t, handler, http.MethodPut, "/api/looks/"+created.ID, cookie, updatePayload)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated := decodeLook(t, rec)

	if updated.ID != created.ID {
		t.Fatalf("update: expected same id, got %q vs %q", updated.ID, created.ID)
	}
	if updated.Name != "Updated Name" || updated.Occasion != "Formal" {
		t.Fatalf("update: expected updated name/occasion, got %+v", updated)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("update: expected createdAt unchanged, got %v vs %v", updated.CreatedAt, created.CreatedAt)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("update: expected updatedAt to advance, got %v (was %v)", updated.UpdatedAt, created.UpdatedAt)
	}

	if len(updated.Items) != 2 {
		t.Fatalf("update: expected 2 items, got %d: %+v", len(updated.Items), updated.Items)
	}
	byItemID := make(map[string]model.LookItem, len(updated.Items))
	for _, item := range updated.Items {
		byItemID[item.ItemID] = item
	}
	if _, present := byItemID[strconv.FormatInt(removedItem, 10)]; present {
		t.Fatalf("update: expected removed item to be gone, got %+v", updated.Items)
	}
	kept, ok := byItemID[strconv.FormatInt(keptItem, 10)]
	if !ok {
		t.Fatalf("update: expected kept item to still be present, got %+v", updated.Items)
	}
	if kept.X != 99 || kept.Y != 88 || kept.Width != 50 || kept.Height != 60 || kept.Rotation != 45 || kept.ZIndex != 1 {
		t.Fatalf("update: kept item transform not updated: %+v", kept)
	}
	added, ok := byItemID[strconv.FormatInt(addedItem, 10)]
	if !ok {
		t.Fatalf("update: expected added item to be present, got %+v", updated.Items)
	}
	if added.ZIndex != 0 {
		t.Fatalf("update: expected re-ordered zIndex to persist, got %+v", added)
	}

	// Directly assert no orphaned LookItems row remains for the removed item.
	var count int
	numericLookID, err := strconv.ParseInt(created.ID, 10, 64)
	if err != nil {
		t.Fatalf("parse look id: %v", err)
	}
	if err := database.QueryRow(
		context.Background(),
		`SELECT count(*) FROM LookItems WHERE lookId = $1 AND inventoryId = $2`,
		numericLookID, removedItem,
	).Scan(&count); err != nil {
		t.Fatalf("query LookItems: %v", err)
	}
	if count != 0 {
		t.Fatalf("update: expected no LookItems row for removed item, found %d", count)
	}

	// A subsequent GET reflects the same state (response is the Get-shaped
	// look, not something update-specific).
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+created.ID, cookie, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get after update: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	fetched := decodeLook(t, rec)
	if fetched.Name != "Updated Name" || len(fetched.Items) != 2 {
		t.Fatalf("get after update: unexpected look: %+v", fetched)
	}
}

// TestLookUpdateValidation mirrors TestLookCreateValidation: the same 400
// cases (blank name, empty items, an itemId the caller doesn't own) apply to
// Update.
func TestLookUpdateValidation(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	userA, cookieA := newTestUser(t, database)
	itemA := newTestItem(t, database, userA, "Own Shoes")
	userB, _ := newTestUser(t, database)
	itemB := newTestItem(t, database, userB, "Someone Else's Bag")

	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookieA, model.Look{
		Name:  "Editable Look",
		Items: []model.LookItem{{ItemID: strconv.FormatInt(itemA, 10), Width: 10, Height: 10}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	created := decodeLook(t, rec)

	cases := []struct {
		name string
		body model.Look
	}{
		{
			name: "blank name",
			body: model.Look{
				Name:  "   ",
				Items: []model.LookItem{{ItemID: strconv.FormatInt(itemA, 10), Width: 10, Height: 10}},
			},
		},
		{
			name: "empty items",
			body: model.Look{
				Name:  "No Pieces",
				Items: []model.LookItem{},
			},
		},
		{
			name: "foreign itemId",
			body: model.Look{
				Name:  "Borrowed Look",
				Items: []model.LookItem{{ItemID: strconv.FormatInt(itemB, 10), Width: 10, Height: 10}},
			},
		},
		{
			name: "unknown itemId",
			body: model.Look{
				Name:  "Made Up Look",
				Items: []model.LookItem{{ItemID: "99999999999", Width: 10, Height: 10}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, handler, http.MethodPut, "/api/looks/"+created.ID, cookieA, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}

	// None of the rejected payloads should have touched the look: a
	// mid-validation failure (before the transaction even opens for some
	// cases, and rolled back for others) must leave the previous items
	// intact, never a partially-wiped look.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+created.ID, cookieA, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get after rejected updates: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	fetched := decodeLook(t, rec)
	if fetched.Name != "Editable Look" || len(fetched.Items) != 1 || fetched.Items[0].ItemID != strconv.FormatInt(itemA, 10) {
		t.Fatalf("get after rejected updates: expected look unchanged, got %+v", fetched)
	}
}

// TestLookUpdateNotFound covers the three 404 paths (unknown id, a
// non-numeric id, another user's look) and proves the last one leaves the
// other user's look byte-for-byte unchanged.
func TestLookUpdateNotFound(t *testing.T) {
	database := testDB(t)
	handler := newLookRouter(database)

	userA, cookieA := newTestUser(t, database)
	itemA := newTestItem(t, database, userA, "A's Shoes")

	userB, cookieB := newTestUser(t, database)
	itemB := newTestItem(t, database, userB, "B's Hat")

	rec := doJSON(t, handler, http.MethodPost, "/api/looks/", cookieB, model.Look{
		Name:     "B's Look",
		Occasion: "Work",
		Items:    []model.LookItem{{ItemID: strconv.FormatInt(itemB, 10), X: 1, Y: 1, Width: 10, Height: 10, ZIndex: 0}},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create for user B: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	bLook := decodeLook(t, rec)

	updatePayload := model.Look{
		Name:  "Hijacked",
		Items: []model.LookItem{{ItemID: strconv.FormatInt(itemA, 10), Width: 10, Height: 10}},
	}

	// Unknown id.
	rec = doJSON(t, handler, http.MethodPut, "/api/looks/9999999999", cookieA, updatePayload)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown id: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// Non-numeric id.
	rec = doJSON(t, handler, http.MethodPut, "/api/looks/not-a-number", cookieA, updatePayload)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("non-numeric id: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// Another user's look: 404, never 403, never a silent no-op.
	rec = doJSON(t, handler, http.MethodPut, "/api/looks/"+bLook.ID, cookieA, updatePayload)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("another user's look: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	// B's look must be provably unchanged.
	rec = doJSON(t, handler, http.MethodGet, "/api/looks/"+bLook.ID, cookieB, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get B's look after attack: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	stillB := decodeLook(t, rec)
	if stillB.Name != bLook.Name || stillB.Occasion != bLook.Occasion {
		t.Fatalf("B's look name/occasion changed: before %+v, after %+v", bLook, stillB)
	}
	if !stillB.UpdatedAt.Equal(bLook.UpdatedAt) {
		t.Fatalf("B's look updatedAt changed: before %v, after %v", bLook.UpdatedAt, stillB.UpdatedAt)
	}
	if len(stillB.Items) != 1 || stillB.Items[0].ItemID != strconv.FormatInt(itemB, 10) {
		t.Fatalf("B's look items changed: before %+v, after %+v", bLook.Items, stillB.Items)
	}
}
