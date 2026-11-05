package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"organizing-app-backend/internal/model"
)

// newCategoryTestService connects to TEST_DATABASE_URL, builds migrations
// 0001..latest in a throwaway schema (dropped on cleanup) and returns a
// service plus two user ids. Skipped when TEST_DATABASE_URL is unset.
func newCategoryTestService(t *testing.T) (CategoryService, int64, int64) {
	t.Helper()
	pool, a, b := newCategoryTestPool(t)
	return NewCategoryService(pool, nil), a, b
}

// newCategoryTestPool is the pool-returning core of newCategoryTestService.
func newCategoryTestPool(t *testing.T) (*pgxpool.Pool, int64, int64) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	schema := "test_" + hex.EncodeToString(buf)

	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema: %v", err)
		}
		admin.Close()
	})

	files, err := filepath.Glob(filepath.Join("..", "db", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations: %v", err)
	}
	sort.Strings(files)
	for _, f := range files {
		script, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(script)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}

	var a, b int64
	for _, id := range []*int64{&a, &b} {
		if err := pool.QueryRow(ctx,
			`INSERT INTO Users (name, createdAt, updatedAt) VALUES ('u', now(), now()) RETURNING id`).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	return pool, a, b
}

func strp(s string) *string { return &s }

func wantGroup(t *testing.T, got *string, want string) {
	t.Helper()
	switch {
	case want == "" && got != nil:
		t.Fatalf("group = %q, want NULL", *got)
	case want != "" && (got == nil || *got != want):
		t.Fatalf("group = %v, want %q", got, want)
	}
}

func TestCreateCategoryStylebookGroup(t *testing.T) {
	svc, u, _ := newCategoryTestService(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		group *string
		want  string
	}{
		{"Shoes", nil, "shoes"},
		{"Pantry", nil, ""},
		{"Hats", strp("bags"), "bags"},
		{"Footwear", strp(""), ""},
	}
	for _, tc := range cases {
		c, err := svc.CreateCategory(ctx, u, tc.name, "", "", tc.group)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		wantGroup(t, c.StylebookGroup, tc.want)
	}
}

func TestUpdateCategoryStylebookGroup(t *testing.T) {
	svc, u, other := newCategoryTestService(t)
	ctx := context.Background()
	c, err := svc.CreateCategory(ctx, u, "Shoes", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.UpdateCategory(ctx, u, c.ID, "Kicks", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	wantGroup(t, got.StylebookGroup, "shoes") // rename keeps group

	got, err = svc.UpdateCategory(ctx, u, c.ID, "Kicks", "", "", strp("bags"))
	if err != nil {
		t.Fatal(err)
	}
	wantGroup(t, got.StylebookGroup, "bags")

	got, err = svc.UpdateCategory(ctx, u, c.ID, "Kicks", "", "", strp(""))
	if err != nil {
		t.Fatal(err)
	}
	wantGroup(t, got.StylebookGroup, "")

	if _, err := svc.UpdateCategory(ctx, other, c.ID, "Kicks", "", "", strp("bags")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other user's update err = %v, want ErrNotFound", err)
	}
}

func TestListCategoriesStylebookGroup(t *testing.T) {
	pool, u, _ := newCategoryTestPool(t)
	svc := NewCategoryService(pool, nil)
	ctx := context.Background()
	for _, n := range []string{"Pantry", "Gear"} {
		if _, err := svc.CreateCategory(ctx, u, n, "", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx,
		`UPDATE Categories SET stylebookGroup = 'bags' WHERE userId = $1 AND name = 'Gear'`, u); err != nil {
		t.Fatal(err)
	}
	cats, err := svc.ListCategories(ctx, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 {
		t.Fatalf("got %d categories, want 2", len(cats))
	}
	for _, c := range cats {
		switch c.Name {
		case "Gear":
			wantGroup(t, c.StylebookGroup, "bags")
		case "Pantry":
			wantGroup(t, c.StylebookGroup, "")
		default:
			t.Fatalf("unexpected category %q", c.Name)
		}
	}
}

func TestInventoryImplicitCategoryStylebookGroup(t *testing.T) {
	pool, u, _ := newCategoryTestPool(t)
	svc := NewInventoryService(pool, nil)
	ctx := context.Background()

	groupOf := func(name string) (*string, int) {
		var g *string
		var n int
		rows, err := pool.Query(ctx, `SELECT stylebookGroup FROM Categories WHERE userId = $1 AND name = $2`, u, name)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			if err := rows.Scan(&g); err != nil {
				t.Fatal(err)
			}
			n++
		}
		return g, n
	}

	for _, tc := range []struct{ cat, want string }{{"Running Shoes", "shoes"}, {"Swimwear", "clothes"}, {"Sneakers", ""}, {"Pantry", ""}} {
		if _, err := svc.Create(ctx, u, model.InventoryItem{Name: "x", Category: tc.cat, Subcategory: "Sub", Quantity: 1}); err != nil {
			t.Fatalf("%s: %v", tc.cat, err)
		}
		g, n := groupOf(tc.cat)
		if n != 1 {
			t.Fatalf("%s: %d rows, want 1", tc.cat, n)
		}
		wantGroup(t, g, tc.want)
	}

	// An existing category keeps its group when another item is saved into it.
	if _, err := pool.Exec(ctx,
		`UPDATE Categories SET stylebookGroup = 'bags' WHERE userId = $1 AND name = 'Running Shoes'`, u); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, u, model.InventoryItem{Name: "y", Category: "running shoes", Subcategory: "Other", Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	g, n := groupOf("Running Shoes")
	if n != 1 {
		t.Fatalf("Running Shoes: %d rows, want 1", n)
	}
	wantGroup(t, g, "bags")
}
