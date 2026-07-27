package service

import (
	"database/sql"
	"errors"

	"organizing-app-backend/internal/model"
)

var ErrNotFound = errors.New("record not found")

// CategoryService manages categories, their subcategories, and item tags.
type CategoryService interface {
	ListCategories() ([]model.Category, error)
	CreateCategory(name, icon, colour string) (model.Category, error)
	UpdateCategory(id int64, name, icon, colour string) (model.Category, error)
	DeleteCategory(id int64) error
	CreateSubCategory(categoryID int64, name string) (model.SubCategory, error)
	DeleteSubCategory(id int64) error
	ListTags() ([]model.ItemTag, error)
	CreateTag(name, colour string) (model.ItemTag, error)
	DeleteTag(id int64) error
	// AttachTag links a tag (created by name if missing) to a category.
	AttachTag(categoryID int64, name, colour string) (model.ItemTag, error)
	// DetachTag removes the category-tag link only; the tag itself survives.
	DetachTag(categoryID, tagID int64) error
}

type postgresCategoryService struct {
	db *sql.DB
}

func NewCategoryService(db *sql.DB) CategoryService {
	return &postgresCategoryService{db: db}
}

func (s *postgresCategoryService) ListCategories() ([]model.Category, error) {
	rows, err := s.db.Query(
		`SELECT id, userId, name, icon, colour, reminderOnExpiry, createdAt, updatedAt FROM Categories ORDER BY id`,
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
		`SELECT id, userId, name, icon, categoryId, createdAt, updatedAt FROM SubCategories ORDER BY id`,
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
	if err := s.hydrateItemStats(categories, byID); err != nil {
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

// hydrateItemStats fills each category's item count and distinct locations.
func (s *postgresCategoryService) hydrateItemStats(categories []model.Category, byID map[int64]int) error {
	countRows, err := s.db.Query(
		`SELECT sc.categoryId, COUNT(*)
		 FROM Inventories i
		 JOIN SubCategories sc ON sc.id = i.subCategoryId
		 GROUP BY sc.categoryId`,
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
		`SELECT DISTINCT sc.categoryId, i.storageLocation
		 FROM Inventories i
		 JOIN SubCategories sc ON sc.id = i.subCategoryId
		 WHERE i.storageLocation IS NOT NULL AND i.storageLocation <> ''
		 ORDER BY sc.categoryId, i.storageLocation`,
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

func (s *postgresCategoryService) CreateCategory(name, icon, colour string) (model.Category, error) {
	var c model.Category
	err := s.db.QueryRow(
		`INSERT INTO Categories (userId, name, icon, colour, createdAt, updatedAt)
		 VALUES (NULL, $1, NULLIF($2, ''), NULLIF($3, ''), now(), now())
		 RETURNING id, userId, name, icon, colour, reminderOnExpiry, createdAt, updatedAt`,
		name, icon, colour,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Icon, &c.Colour, &c.ReminderOnExpiry, &c.CreatedAt, &c.UpdatedAt)
	c.SubCategories = make([]model.SubCategory, 0)
	c.Tags = make([]model.ItemTag, 0)
	c.Locations = make([]string, 0)
	return c, err
}

// UpdateCategory sets the category's name and, when non-empty, its icon and
// colour. Empty icon/colour leave the stored values unchanged.
func (s *postgresCategoryService) UpdateCategory(id int64, name, icon, colour string) (model.Category, error) {
	var c model.Category
	err := s.db.QueryRow(
		`UPDATE Categories
		 SET name = $1,
		     icon = COALESCE(NULLIF($2, ''), icon),
		     colour = COALESCE(NULLIF($3, ''), colour),
		     updatedAt = now()
		 WHERE id = $4
		 RETURNING id, userId, name, icon, colour, reminderOnExpiry, createdAt, updatedAt`,
		name, icon, colour, id,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Icon, &c.Colour, &c.ReminderOnExpiry, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Category{}, ErrNotFound
	}
	c.SubCategories = make([]model.SubCategory, 0)
	c.Tags = make([]model.ItemTag, 0)
	c.Locations = make([]string, 0)
	return c, err
}

func (s *postgresCategoryService) DeleteCategory(id int64) error {
	return s.deleteByID(`DELETE FROM Categories WHERE id = $1`, id)
}

func (s *postgresCategoryService) CreateSubCategory(categoryID int64, name string) (model.SubCategory, error) {
	var sc model.SubCategory
	err := s.db.QueryRow(
		`INSERT INTO SubCategories (userId, name, categoryId, createdAt, updatedAt)
		 VALUES (NULL, $1, $2, now(), now())
		 RETURNING id, userId, name, icon, categoryId, createdAt, updatedAt`,
		name, categoryID,
	).Scan(&sc.ID, &sc.UserID, &sc.Name, &sc.Icon, &sc.CategoryID, &sc.CreatedAt, &sc.UpdatedAt)
	return sc, err
}

func (s *postgresCategoryService) DeleteSubCategory(id int64) error {
	return s.deleteByID(`DELETE FROM SubCategories WHERE id = $1`, id)
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

func (s *postgresCategoryService) DetachTag(categoryID, tagID int64) error {
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
	return nil
}

func (s *postgresCategoryService) DeleteTag(id int64) error {
	return s.deleteByID(`DELETE FROM ItemTags WHERE id = $1`, id)
}

func (s *postgresCategoryService) deleteByID(query string, id int64) error {
	result, err := s.db.Exec(query, id)
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
