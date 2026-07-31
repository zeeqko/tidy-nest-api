package service

import (
	"database/sql"
	"errors"
)

// defaultSubCategorySeed is one subcategory row to create under its parent
// defaultCategorySeed.
type defaultSubCategorySeed struct {
	name string
	icon string
}

// defaultTagSeed is a tag to find-or-create and link to a category via
// CategoryTags. colour is only used the first time the tag is created (see
// attachDefaultTag); an existing tag with the same name keeps its own colour.
type defaultTagSeed struct {
	name   string
	colour string
}

// defaultCategorySeed is one category row plus its subcategories and tags.
type defaultCategorySeed struct {
	name             string
	icon             string
	reminderOnExpiry bool
	subCategories    []defaultSubCategorySeed
	tags             []defaultTagSeed
}

// defaultCategorySeeds is the sole source of truth for a new user's starter
// categories (there's no longer a seeded demo user or global default rows to
// mirror) — order here is the order a fresh signup's `ListCategories`
// (ORDER BY id) will display them in.
var defaultCategorySeeds = []defaultCategorySeed{
	{
		name:             "Food",
		icon:             "food",
		reminderOnExpiry: true,
		subCategories: []defaultSubCategorySeed{
			{name: "Dairy", icon: "milk"},
			{name: "Meat", icon: "beef"},
		},
		tags: []defaultTagSeed{
			{name: "Fresh", colour: "#B8EFC0"},
			{name: "Frozen", colour: "#D6ECFF"},
		},
	},
	{
		name:             "Clothes",
		icon:             "clothes",
		reminderOnExpiry: false,
		subCategories: []defaultSubCategorySeed{
			{name: "Outerwear", icon: "shirt"},
			{name: "Tops", icon: "shirt"},
		},
		tags: []defaultTagSeed{
			{name: "Winter", colour: "#D6ECFF"},
			{name: "Summer", colour: "#FFE9A8"},
		},
	},
	{
		name:             "Makeup",
		icon:             "makeup",
		reminderOnExpiry: true,
		subCategories: []defaultSubCategorySeed{
			{name: "Cosmetics", icon: "sparkles"},
		},
	},
	{
		name:             "Shoes",
		icon:             "shoes",
		reminderOnExpiry: false,
		subCategories: []defaultSubCategorySeed{
			{name: "Sneakers", icon: "footprints"},
		},
	},
	{
		name:             "Bags",
		icon:             "bags",
		reminderOnExpiry: false,
		subCategories: []defaultSubCategorySeed{
			{name: "Handbags", icon: "shopping-bag"},
		},
	},
	{
		name:             "Books",
		icon:             "books",
		reminderOnExpiry: false,
		subCategories: []defaultSubCategorySeed{
			{name: "Fiction", icon: "book-open"},
		},
	},
}

// seedDefaultCategories gives userID its own copy of the default
// categories/subcategories (no `colour`, left NULL) and links the
// pre-existing global ItemTags (Fresh/Frozen/Winter/Summer) to the
// user's Food/Clothes categories, find-or-creating those tags by name so
// signup never duplicates a global tag row (mirrors AttachTag's semantics in
// category_service.go). Runs inside tx so a failure rolls back the whole
// signup with it.
func seedDefaultCategories(tx *sql.Tx, userID int64) error {
	for _, dc := range defaultCategorySeeds {
		var categoryID int64
		if err := tx.QueryRow(
			`INSERT INTO Categories (userId, name, icon, reminderOnExpiry, createdAt, updatedAt)
			 VALUES ($1, $2, $3, $4, now(), now())
			 RETURNING id`,
			userID, dc.name, dc.icon, dc.reminderOnExpiry,
		).Scan(&categoryID); err != nil {
			return err
		}

		for _, sc := range dc.subCategories {
			if _, err := tx.Exec(
				`INSERT INTO SubCategories (userId, name, icon, categoryId, createdAt, updatedAt)
				 VALUES ($1, $2, $3, $4, now(), now())`,
				userID, sc.name, sc.icon, categoryID,
			); err != nil {
				return err
			}
		}

		for _, tag := range dc.tags {
			if err := attachDefaultTag(tx, categoryID, tag.name, tag.colour); err != nil {
				return err
			}
		}
	}
	return nil
}

// attachDefaultTag finds the global ItemTags row by name, creating it only
// if missing, then links it to categoryID via CategoryTags. Mirrors
// AttachTag's find-or-create semantics (category_service.go) so re-running
// signup logic never produces a second ItemTags row for the same name.
//
// ItemTags has a partial unique index on lower(name) for NULL-owner rows
// (migration 0011), so two concurrent signups racing to create the same tag
// name can't both succeed: the INSERT below targets that index as its ON
// CONFLICT clause, so the losing transaction's INSERT is a no-op (RETURNING
// yields no row) rather than a duplicate row or an error, and it falls back
// to the SELECT to pick up the winning transaction's committed row.
func attachDefaultTag(tx *sql.Tx, categoryID int64, name, colour string) error {
	var tagID int64
	err := tx.QueryRow(
		`INSERT INTO ItemTags (userId, name, colour, createdAt, updatedAt)
		 VALUES (NULL, $1, $2, now(), now())
		 ON CONFLICT (lower(name)) WHERE userId IS NULL DO NOTHING
		 RETURNING id`,
		name, colour,
	).Scan(&tagID)
	if errors.Is(err, sql.ErrNoRows) {
		// Exact-case, not scoped to userId IS NULL: matches AttachTag's
		// existing find-or-create SELECT (category_service.go) so this stays
		// consistent with the rest of the codebase's tag lookups.
		err = tx.QueryRow(`SELECT id FROM ItemTags WHERE name = $1 ORDER BY id LIMIT 1`, name).Scan(&tagID)
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		`INSERT INTO CategoryTags (categoryId, tagId) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		categoryID, tagID,
	)
	return err
}
