package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgconn"

	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/storage"
)

var (
	ErrNotFound = errors.New("record not found")
	// ErrDuplicateName marks a case-insensitive category/subcategory name
	// collision (Postgres unique-violation, SQLSTATE 23505), surfaced by the
	// controller as 409. Errors returned for a specific violation carry a
	// more specific message but still satisfy errors.Is(err, ErrDuplicateName).
	ErrDuplicateName = errors.New("a category or subcategory with this name already exists")
)

// CategoryService manages categories, their subcategories, and item tags.
// Categories and subcategories are scoped to the owning user; tags are not
// yet (see PLAN.md T1 open question #6) and remain global.
type CategoryService interface {
	ListCategories(userID int64) ([]model.Category, error)
	CreateCategory(userID int64, name, icon, colour string) (model.Category, error)
	UpdateCategory(userID, id int64, name, icon, colour string) (model.Category, error)
	DeleteCategory(userID, id int64) error
	CreateSubCategory(userID, categoryID int64, name string) (model.SubCategory, error)
	DeleteSubCategory(userID, id int64) error
	ListTags() ([]model.ItemTag, error)
	CreateTag(name, colour string) (model.ItemTag, error)
	DeleteTag(id int64) error
	// AttachTag links a tag (created by name if missing) to a category.
	AttachTag(categoryID int64, name, colour string) (model.ItemTag, error)
	// DetachTag removes the tag from the category (and from userID's items
	// filed under it); the tag itself, and its use elsewhere, survives.
	DetachTag(userID, categoryID, tagID int64) error
}

// duplicateNameError carries a message specific to the violated constraint
// while still matching errors.Is(err, ErrDuplicateName).
type duplicateNameError struct{ message string }

func (e *duplicateNameError) Error() string        { return e.message }
func (e *duplicateNameError) Is(target error) bool { return target == ErrDuplicateName }

func newDuplicateNameError(message string) error {
	return &duplicateNameError{message: message}
}

// isDuplicateNameViolation reports whether err is a Postgres unique-violation
// (SQLSTATE 23505), i.e. the categories_user_name_unique or
// subcategories_user_category_name_unique index rejected the write.
func isDuplicateNameViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// isForeignKeyViolation reports whether err is a Postgres foreign-key
// violation (SQLSTATE 23503). Used to catch the narrow TOCTOU window between
// CreateSubCategory's ownership check and its insert: if the category is
// deleted in between, the insert fails this way instead of racily succeeding.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

type postgresCategoryService struct {
	db    *sql.DB
	store storage.Store
}

func NewCategoryService(db *sql.DB, store storage.Store) CategoryService {
	return &postgresCategoryService{db: db, store: store}
}

func (s *postgresCategoryService) ListCategories(userID int64) ([]model.Category, error) {
	rows, err := s.db.Query(
		`SELECT id, userId, name, icon, colour, reminderOnExpiry, createdAt, updatedAt FROM Categories WHERE userId = $1 ORDER BY id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]model.Category, 0)
	byID := make(map[int64]int)
	for rows.Next() {
		var c model.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Icon, &c.Colour, &c.ReminderOnExpiry, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.SubCategories = make([]model.SubCategory, 0)
		c.Tags = make([]model.ItemTag, 0)
		c.Locations = make([]string, 0)
		byID[c.ID] = len(categories)
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	subRows, err := s.db.Query(
		`SELECT id, userId, name, icon, categoryId, createdAt, updatedAt FROM SubCategories WHERE userId = $1 ORDER BY id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer subRows.Close()

	for subRows.Next() {
		var sc model.SubCategory
		if err := subRows.Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.Icon, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		if idx, ok := byID[sc.CategoryID]; ok {
			categories[idx].SubCategories = append(categories[idx].SubCategories, sc)
		}
	}
	if err := subRows.Err(); err != nil {
		return nil, err
	}

	if err := s.hydrateTags(categories, byID); err != nil {
		return nil, err
	}
	if err := s.hydrateItemStats(categories, byID, userID); err != nil {
		return nil, err
	}
	return categories, nil
}

// hydrateTags nests each category's tags (with their full category link list)
// so clients get everything from the categories endpoint alone.
func (s *postgresCategoryService) hydrateTags(categories []model.Category, byID map[int64]int) error {
	rows, err := s.db.Query(
		`SELECT ct.categoryId, t.id, t.userId, t.name, t.colour, t.createdAt, t.updatedAt
		 FROM CategoryTags ct
		 JOIN ItemTags t ON t.id = ct.tagId
		 ORDER BY ct.categoryId, t.id`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type link struct {
		categoryID int64
		tag        model.ItemTag
	}
	links := make([]link, 0)
	tagCategories := make(map[int64][]int64)
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.categoryID, &l.tag.ID, &l.tag.UserID, &l.tag.Name, &l.tag.Colour, &l.tag.CreatedAt, &l.tag.UpdatedAt); err != nil {
			return err
		}
		tagCategories[l.tag.ID] = append(tagCategories[l.tag.ID], l.categoryID)
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, l := range links {
		l.tag.CategoryIDs = tagCategories[l.tag.ID]
		if idx, ok := byID[l.categoryID]; ok {
			categories[idx].Tags = append(categories[idx].Tags, l.tag)
		}
	}
	return nil
}

// hydrateItemStats fills each category's item count and distinct locations,
// scoped to userID's own inventory items. Both are keyed off Inventories'
// own categoryId column (not subCategoryId), so an item whose subcategory
// was since deleted still counts toward its category.
func (s *postgresCategoryService) hydrateItemStats(categories []model.Category, byID map[int64]int, userID int64) error {
	countRows, err := s.db.Query(
		`SELECT categoryId, COUNT(*)
		 FROM Inventories
		 WHERE userId = $1 AND categoryId IS NOT NULL
		 GROUP BY categoryId`,
		userID,
	)
	if err != nil {
		return err
	}
	defer countRows.Close()

	for countRows.Next() {
		var categoryID int64
		var count int
		if err := countRows.Scan(&categoryID, &count); err != nil {
			return err
		}
		if idx, ok := byID[categoryID]; ok {
			categories[idx].ItemCount = count
		}
	}
	if err := countRows.Err(); err != nil {
		return err
	}

	locationRows, err := s.db.Query(
		`SELECT DISTINCT categoryId, storageLocation
		 FROM Inventories
		 WHERE userId = $1 AND categoryId IS NOT NULL AND storageLocation IS NOT NULL AND storageLocation <> ''
		 ORDER BY categoryId, storageLocation`,
		userID,
	)
	if err != nil {
		return err
	}
	defer locationRows.Close()

	for locationRows.Next() {
		var categoryID int64
		var location string
		if err := locationRows.Scan(&categoryID, &location); err != nil {
			return err
		}
		if idx, ok := byID[categoryID]; ok {
			categories[idx].Locations = append(categories[idx].Locations, location)
		}
	}
	return locationRows.Err()
}

func (s *postgresCategoryService) CreateCategory(userID int64, name, icon, colour string) (model.Category, error) {
	var c model.Category
	err := s.db.QueryRow(
		`INSERT INTO Categories (userId, name, icon, colour, createdAt, updatedAt)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), now(), now())
		 RETURNING id, userId, name, icon, colour, reminderOnExpiry, createdAt, updatedAt`,
		userID, name, icon, colour,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Icon, &c.Colour, &c.ReminderOnExpiry, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if isDuplicateNameViolation(err) {
			return model.Category{}, newDuplicateNameError(fmt.Sprintf("a category named %q already exists", name))
		}
		return model.Category{}, err
	}
	c.SubCategories = make([]model.SubCategory, 0)
	c.Tags = make([]model.ItemTag, 0)
	c.Locations = make([]string, 0)
	return c, nil
}

// UpdateCategory sets the category's name and, when non-empty, its icon and
// colour. Empty icon/colour leave the stored values unchanged. Only the
// owning user's category can be updated; any other id (missing or belonging
// to someone else) reports ErrNotFound.
func (s *postgresCategoryService) UpdateCategory(userID, id int64, name, icon, colour string) (model.Category, error) {
	var c model.Category
	err := s.db.QueryRow(
		`UPDATE Categories
		 SET name = $1,
		     icon = COALESCE(NULLIF($2, ''), icon),
		     colour = COALESCE(NULLIF($3, ''), colour),
		     updatedAt = now()
		 WHERE id = $4 AND userId = $5
		 RETURNING id, userId, name, icon, colour, reminderOnExpiry, createdAt, updatedAt`,
		name, icon, colour, id, userID,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Icon, &c.Colour, &c.ReminderOnExpiry, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Category{}, ErrNotFound
	}
	if err != nil {
		if isDuplicateNameViolation(err) {
			return model.Category{}, newDuplicateNameError(fmt.Sprintf("a category named %q already exists", name))
		}
		return model.Category{}, err
	}
	c.SubCategories = make([]model.SubCategory, 0)
	c.Tags = make([]model.ItemTag, 0)
	c.Locations = make([]string, 0)
	return c, nil
}

// DeleteCategory deletes the category, which cascades (see migration 0012)
// to its subcategories and, crucially, to every item filed directly under it
// (Inventories.categoryId ON DELETE CASCADE) — items don't outlive their
// category the way they survive a subcategory or tag deletion. Since that
// cascade bypasses InventoryService.Delete, any item photos are cleaned up
// here instead.
func (s *postgresCategoryService) DeleteCategory(userID, id int64) error {
	imageURLs, err := s.itemImageURLs(userID, id)
	if err != nil {
		return err
	}
	if err := s.deleteByID(`DELETE FROM Categories WHERE id = $1 AND userId = $2`, id, userID); err != nil {
		return err
	}
	s.cleanupImages(imageURLs)
	return nil
}

func (s *postgresCategoryService) itemImageURLs(userID, categoryID int64) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT imageURL FROM Inventories WHERE userId = $1 AND categoryId = $2 AND imageURL IS NOT NULL`,
		userID, categoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := make([]string, 0)
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, rows.Err()
}

func (s *postgresCategoryService) cleanupImages(imageURLs []string) {
	for _, url := range imageURLs {
		key, ok := uploadKey(url)
		if !ok {
			continue
		}
		if err := s.store.Delete(context.Background(), key); err != nil {
			log.Printf("delete item photo %s (category cascade): %v", key, err)
		}
	}
}

// CreateSubCategory verifies categoryID belongs to userID before inserting,
// so a subcategory can never be attached to another user's category.
func (s *postgresCategoryService) CreateSubCategory(userID, categoryID int64, name string) (model.SubCategory, error) {
	var exists bool
	if err := s.db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM Categories WHERE id = $1 AND userId = $2)`, categoryID, userID,
	).Scan(&exists); err != nil {
		return model.SubCategory{}, err
	}
	if !exists {
		return model.SubCategory{}, ErrNotFound
	}

	var sc model.SubCategory
	err := s.db.QueryRow(
		`INSERT INTO SubCategories (userId, name, categoryId, createdAt, updatedAt)
		 VALUES ($1, $2, $3, now(), now())
		 RETURNING id, userId, name, icon, categoryId, createdAt, updatedAt`,
		userID, name, categoryID,
	).Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.Icon, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		if isDuplicateNameViolation(err) {
			return model.SubCategory{}, newDuplicateNameError(fmt.Sprintf("a subcategory named %q already exists in this category", name))
		}
		if isForeignKeyViolation(err) {
			return model.SubCategory{}, ErrNotFound
		}
		return model.SubCategory{}, err
	}
	return sc, nil
}

func (s *postgresCategoryService) DeleteSubCategory(userID, id int64) error {
	return s.deleteByID(`DELETE FROM SubCategories WHERE id = $1 AND userId = $2`, id, userID)
}

func (s *postgresCategoryService) ListTags() ([]model.ItemTag, error) {
	rows, err := s.db.Query(`SELECT id, userId, name, colour, createdAt, updatedAt FROM ItemTags ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := make([]model.ItemTag, 0)
	byID := make(map[int64]int)
	for rows.Next() {
		var t model.ItemTag
		if err := rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Colour, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.CategoryIDs = make([]int64, 0)
		byID[t.ID] = len(tags)
		tags = append(tags, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	linkRows, err := s.db.Query(`SELECT tagId, categoryId FROM CategoryTags ORDER BY categoryId`)
	if err != nil {
		return nil, err
	}
	defer linkRows.Close()

	for linkRows.Next() {
		var tagID, categoryID int64
		if err := linkRows.Scan(&tagID, &categoryID); err != nil {
			return nil, err
		}
		if idx, ok := byID[tagID]; ok {
			tags[idx].CategoryIDs = append(tags[idx].CategoryIDs, categoryID)
		}
	}
	return tags, linkRows.Err()
}

func (s *postgresCategoryService) CreateTag(name, colour string) (model.ItemTag, error) {
	if colour == "" {
		colour = randomPastelColour()
	}
	var t model.ItemTag
	err := s.db.QueryRow(
		`INSERT INTO ItemTags (userId, name, colour, createdAt, updatedAt)
		 VALUES (NULL, $1, $2, now(), now())
		 RETURNING id, userId, name, colour, createdAt, updatedAt`,
		name, colour,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Colour, &t.CreatedAt, &t.UpdatedAt)
	t.CategoryIDs = make([]int64, 0)
	return t, err
}

func (s *postgresCategoryService) AttachTag(categoryID int64, name, colour string) (model.ItemTag, error) {
	var exists bool
	if err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM Categories WHERE id = $1)`, categoryID).Scan(&exists); err != nil {
		return model.ItemTag{}, err
	}
	if !exists {
		return model.ItemTag{}, ErrNotFound
	}

	var t model.ItemTag
	err := s.db.QueryRow(`SELECT id, userId, name, colour, createdAt, updatedAt FROM ItemTags WHERE name = $1 ORDER BY id LIMIT 1`, name).
		Scan(&t.ID, &t.UserID, &t.Name, &t.Colour, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		t, err = s.CreateTag(name, colour)
	}
	if err != nil {
		return model.ItemTag{}, err
	}

	if _, err := s.db.Exec(
		`INSERT INTO CategoryTags (categoryId, tagId) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		categoryID, t.ID,
	); err != nil {
		return model.ItemTag{}, err
	}

	linkRows, err := s.db.Query(`SELECT categoryId FROM CategoryTags WHERE tagId = $1 ORDER BY categoryId`, t.ID)
	if err != nil {
		return model.ItemTag{}, err
	}
	defer linkRows.Close()
	t.CategoryIDs = make([]int64, 0)
	for linkRows.Next() {
		var id int64
		if err := linkRows.Scan(&id); err != nil {
			return model.ItemTag{}, err
		}
		t.CategoryIDs = append(t.CategoryIDs, id)
	}
	return t, linkRows.Err()
}

// DetachTag unlinks the tag from the category and strips it from userID's
// items filed under that category — otherwise an item would keep showing a
// tag its category no longer offers, the same inconsistency deleting a
// subcategory used to leave behind before Inventories.categoryId existed
// (see migration 0012). The tag row itself, and its links to other
// categories/users, are untouched.
func (s *postgresCategoryService) DetachTag(userID, categoryID, tagID int64) error {
	var owned bool
	if err := s.db.QueryRow(
		`SELECT EXISTS (SELECT 1 FROM Categories WHERE id = $1 AND userId = $2)`, categoryID, userID,
	).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return ErrNotFound
	}

	result, err := s.db.Exec(`DELETE FROM CategoryTags WHERE categoryId = $1 AND tagId = $2`, categoryID, tagID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}

	if _, err := s.db.Exec(
		`DELETE FROM InventoryTags
		 WHERE tagId = $1
		   AND inventoryId IN (SELECT id FROM Inventories WHERE userId = $2 AND categoryId = $3)`,
		tagID, userID, categoryID,
	); err != nil {
		return err
	}
	return nil
}

func (s *postgresCategoryService) DeleteTag(id int64) error {
	return s.deleteByID(`DELETE FROM ItemTags WHERE id = $1`, id)
}

func (s *postgresCategoryService) deleteByID(query string, args ...any) error {
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
