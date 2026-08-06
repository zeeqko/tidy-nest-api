package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"organizing-app-backend/internal/model"
	"organizing-app-backend/internal/storage"
)

var ErrItemNotFound = errors.New("inventory item not found")

// InventoryService defines the business operations available on inventory items.
// Every operation is scoped to the authenticated user's id.
type InventoryService interface {
	List(ctx context.Context, userID int64) ([]model.InventoryItem, error)
	Get(ctx context.Context, userID int64, id string) (model.InventoryItem, error)
	Create(ctx context.Context, userID int64, item model.InventoryItem) (model.InventoryItem, error)
	Update(ctx context.Context, userID int64, id string, item model.InventoryItem) (model.InventoryItem, error)
	Delete(ctx context.Context, userID int64, id string) error
}

// postgresInventoryService serves the flat API item shape from the relational
// schema (Inventories → SubCategories → Categories, ItemTags). Subtitle and
// status are derived, not stored, so they are ignored on Create/Update.
type postgresInventoryService struct {
	db    *pgxpool.Pool
	store storage.Store
}

func NewInventoryService(db *pgxpool.Pool, store storage.Store) InventoryService {
	return &postgresInventoryService{db: db, store: store}
}

const selectItem = `
SELECT i.id, i.name, i.quantity,
       COALESCE(i.storageLocation, ''), COALESCE(i.notes, ''), COALESCE(i.imageURL, ''), COALESCE(i.cutoutURL, ''),
       COALESCE(sc.name, ''), COALESCE(c.name, ''), sc.id, c.id,
       i.expiryDate, i.opensOn, i.createdAt, i.updatedAt
FROM Inventories i
LEFT JOIN SubCategories sc ON sc.id = i.subCategoryId
LEFT JOIN Categories c ON c.id = i.categoryId
WHERE i.userId = $1`

func (s *postgresInventoryService) List(ctx context.Context, userID int64) ([]model.InventoryItem, error) {
	rows, err := s.db.Query(ctx, selectItem+` ORDER BY i.createdAt, i.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.InventoryItem, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.fillTags(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *postgresInventoryService) Get(ctx context.Context, userID int64, id string) (model.InventoryItem, error) {
	numericID, err := parseID(id)
	if err != nil {
		return model.InventoryItem{}, ErrItemNotFound
	}

	row := s.db.QueryRow(ctx, selectItem+` AND i.id = $2`, userID, numericID)
	item, err := scanItem(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.InventoryItem{}, ErrItemNotFound
	}
	if err != nil {
		return model.InventoryItem{}, err
	}

	items := []model.InventoryItem{item}
	if err := s.fillTags(ctx, items); err != nil {
		return model.InventoryItem{}, err
	}
	return items[0], nil
}

// fillTags populates Tags on each item from the InventoryTags junction table.
func (s *postgresInventoryService) fillTags(ctx context.Context, items []model.InventoryItem) error {
	byID := make(map[string]*model.InventoryItem, len(items))
	ids := make([]int64, 0, len(items))
	for i := range items {
		items[i].Tags = []model.TagRef{}
		byID[items[i].ID] = &items[i]
		numericID, err := parseID(items[i].ID)
		if err != nil {
			return err
		}
		ids = append(ids, numericID)
	}
	if len(ids) == 0 {
		return nil
	}

	rows, err := s.db.Query(
		ctx,
		`SELECT it.inventoryId, t.name, COALESCE(t.colour, '')
		 FROM InventoryTags it
		 JOIN ItemTags t ON t.id = it.tagId
		 WHERE it.inventoryId = ANY($1)
		 ORDER BY t.name`,
		ids,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			inventoryID int64
			tag         model.TagRef
		)
		if err := rows.Scan(&inventoryID, &tag.Name, &tag.Colour); err != nil {
			return err
		}
		if item := byID[strconv.FormatInt(inventoryID, 10)]; item != nil {
			item.Tags = append(item.Tags, tag)
		}
	}
	return rows.Err()
}

func (s *postgresInventoryService) Create(ctx context.Context, userID int64, item model.InventoryItem) (model.InventoryItem, error) {
	if item.Quantity <= 0 {
		item.Quantity = 1
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.InventoryItem{}, err
	}
	defer tx.Rollback(ctx)

	subCategoryID, categoryID, err := ensureSubCategory(ctx, tx, userID, item.Category, item.Subcategory)
	if err != nil {
		return model.InventoryItem{}, err
	}

	var id int64
	err = tx.QueryRow(
		ctx,
		`INSERT INTO Inventories (userId, name, subCategoryId, categoryId, quantity, storageLocation, notes, imageURL, cutoutURL, expiryDate, opensOn, createdAt, updatedAt)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, '')::date, NULLIF($11, '')::date, now(), now())
		 RETURNING id`,
		userID, item.Name, subCategoryID, categoryID, item.Quantity, item.Location, item.Notes, item.ImageURL, item.CutoutURL, item.ExpiryDate, item.OpensOn,
	).Scan(&id)
	if err != nil {
		return model.InventoryItem{}, err
	}
	if err := replaceItemTags(ctx, tx, id, item.Tags, categoryID); err != nil {
		return model.InventoryItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.InventoryItem{}, err
	}

	return s.Get(ctx, userID, strconv.FormatInt(id, 10))
}

func (s *postgresInventoryService) Update(ctx context.Context, userID int64, id string, item model.InventoryItem) (model.InventoryItem, error) {
	numericID, err := parseID(id)
	if err != nil {
		return model.InventoryItem{}, ErrItemNotFound
	}
	if item.Quantity <= 0 {
		item.Quantity = 1
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.InventoryItem{}, err
	}
	defer tx.Rollback(ctx)

	subCategoryID, categoryID, err := ensureSubCategory(ctx, tx, userID, item.Category, item.Subcategory)
	if err != nil {
		return model.InventoryItem{}, err
	}

	result, err := tx.Exec(
		ctx,
		`UPDATE Inventories
		 SET name = $1, subCategoryId = $2, categoryId = $3, quantity = $4, storageLocation = $5, notes = $6, imageURL = NULLIF($7, ''),
		     cutoutURL = NULLIF($8, ''), expiryDate = NULLIF($9, '')::date, opensOn = NULLIF($10, '')::date, updatedAt = now()
		 WHERE id = $11 AND userId = $12`,
		item.Name, subCategoryID, categoryID, item.Quantity, item.Location, item.Notes, item.ImageURL, item.CutoutURL, item.ExpiryDate, item.OpensOn, numericID, userID,
	)
	if err != nil {
		return model.InventoryItem{}, err
	}
	affected := result.RowsAffected()
	if affected == 0 {
		return model.InventoryItem{}, ErrItemNotFound
	}
	if err := replaceItemTags(ctx, tx, numericID, item.Tags, categoryID); err != nil {
		return model.InventoryItem{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.InventoryItem{}, err
	}

	return s.Get(ctx, userID, id)
}

func (s *postgresInventoryService) Delete(ctx context.Context, userID int64, id string) error {
	numericID, err := parseID(id)
	if err != nil {
		return ErrItemNotFound
	}

	var imageURL, cutoutURL string
	err = s.db.QueryRow(
		ctx,
		`DELETE FROM Inventories WHERE id = $1 AND userId = $2 RETURNING COALESCE(imageURL, ''), COALESCE(cutoutURL, '')`,
		numericID, userID,
	).Scan(&imageURL, &cutoutURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrItemNotFound
	}
	if err != nil {
		return err
	}

	if key, ok := uploadKey(imageURL); ok {
		if err := s.store.Delete(context.Background(), key); err != nil {
			log.Printf("delete item photo %s (item %d): %v", key, numericID, err)
		}
	}
	if key, ok := uploadKey(cutoutURL); ok {
		if err := s.store.Delete(context.Background(), key); err != nil {
			log.Printf("delete item cutout %s (item %d): %v", key, numericID, err)
		}
	}
	return nil
}

// uploadKey extracts the storage key from a stored imageURL, per decision #3:
// only a same-origin "/uploads/<key>" URL (the exact form minted by
// UploadController.Upload) yields a key. Anything else — empty, an absolute
// http(s) URL, or an unrecognized shape — yields no key. A remainder
// containing "/", "\" or ".." is rejected too, mirroring the guard in
// UploadController.Serve, so a malformed stored value can never turn into a
// delete outside the key space.
func uploadKey(imageURL string) (key string, ok bool) {
	const prefix = "/uploads/"
	if !strings.HasPrefix(imageURL, prefix) {
		return "", false
	}
	key = strings.TrimPrefix(imageURL, prefix)
	if key == "" || strings.ContainsAny(key, "/\\") || strings.Contains(key, "..") {
		return "", false
	}
	return key, true
}

// ensureSubCategory resolves (creating if necessary) the subcategory named
// subcategory under the category named category, both scoped to userID, and
// returns the subcategory and category ids. Both names empty yields NULLs.
// Lookups are case-insensitive to match the categories_user_name_unique /
// subcategories_user_category_name_unique indexes, so an existing category
// or subcategory is reused regardless of the casing the caller sent.
func ensureSubCategory(ctx context.Context, tx pgx.Tx, userID int64, category, subcategory string) (subCategoryID, categoryID *int64, err error) {
	if category == "" && subcategory == "" {
		return nil, nil, nil
	}
	if subcategory == "" {
		subcategory = "General"
	}
	if category == "" {
		category = subcategory
	}

	var catID int64
	err = tx.QueryRow(
		ctx,
		`SELECT id FROM Categories WHERE userId = $1 AND lower(name) = lower($2) ORDER BY id LIMIT 1`,
		userID, category,
	).Scan(&catID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(
			ctx,
			`INSERT INTO Categories (userId, name, createdAt, updatedAt) VALUES ($1, $2, now(), now()) RETURNING id`,
			userID, category,
		).Scan(&catID)
	}
	if err != nil {
		return nil, nil, err
	}

	var subCatID int64
	err = tx.QueryRow(
		ctx,
		`SELECT id FROM SubCategories WHERE userId = $1 AND categoryId = $2 AND lower(name) = lower($3) ORDER BY id LIMIT 1`,
		userID, catID, subcategory,
	).Scan(&subCatID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(
			ctx,
			`INSERT INTO SubCategories (userId, name, categoryId, createdAt, updatedAt) VALUES ($1, $2, $3, now(), now()) RETURNING id`,
			userID, subcategory, catID,
		).Scan(&subCatID)
	}
	if err != nil {
		return nil, nil, err
	}
	return &subCatID, &catID, nil
}

// replaceItemTags rewrites the item's tag set: every named tag is resolved
// (created if necessary), linked to the item's category, and attached to the
// item, replacing whatever tags it had before.
func replaceItemTags(ctx context.Context, tx pgx.Tx, inventoryID int64, tags []model.TagRef, categoryID *int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM InventoryTags WHERE inventoryId = $1`, inventoryID); err != nil {
		return err
	}
	for _, tag := range tags {
		tagID, err := ensureTag(ctx, tx, tag.Name, categoryID)
		if err != nil {
			return err
		}
		if tagID == nil {
			continue
		}
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO InventoryTags (inventoryId, tagId) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			inventoryID, *tagID,
		); err != nil {
			return err
		}
	}
	return nil
}

// ensureTag resolves (creating if necessary) the tag by name, returning its
// id, or NULL for an empty name. When a category id is given, the tag is also
// linked to that category via CategoryTags so it appears in its tag list.
func ensureTag(ctx context.Context, tx pgx.Tx, name string, categoryID *int64) (*int64, error) {
	if name == "" {
		return nil, nil
	}

	var tagID int64
	err := tx.QueryRow(ctx, `SELECT id FROM ItemTags WHERE name = $1 ORDER BY id LIMIT 1`, name).Scan(&tagID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(
			ctx,
			`INSERT INTO ItemTags (userId, name, colour, createdAt, updatedAt) VALUES (NULL, $1, $2, now(), now()) RETURNING id`,
			name, randomPastelColour(),
		).Scan(&tagID)
	}
	if err != nil {
		return nil, err
	}

	if categoryID != nil {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO CategoryTags (categoryId, tagId) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			*categoryID, tagID,
		); err != nil {
			return nil, err
		}
	}
	return &tagID, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (model.InventoryItem, error) {
	var (
		item          model.InventoryItem
		id            int64
		subCategoryID sql.NullInt64
		categoryID    sql.NullInt64
		expiry        sql.NullTime
		opens         sql.NullTime
	)
	err := row.Scan(
		&id, &item.Name, &item.Quantity,
		&item.Location, &item.Notes, &item.ImageURL, &item.CutoutURL,
		&item.Subcategory, &item.Category, &subCategoryID, &categoryID,
		&expiry, &opens, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return model.InventoryItem{}, err
	}

	item.ID = strconv.FormatInt(id, 10)
	if subCategoryID.Valid {
		s := strconv.FormatInt(subCategoryID.Int64, 10)
		item.SubCategoryID = &s
	}
	if categoryID.Valid {
		s := strconv.FormatInt(categoryID.Int64, 10)
		item.CategoryID = &s
	}
	item.Status = deriveStatus(expiry)
	if expiry.Valid {
		item.ExpiryDate = expiry.Time.Format("2006-01-02")
	}
	if opens.Valid {
		item.OpensOn = opens.Time.Format("2006-01-02")
	}
	if item.Subcategory != "" {
		item.Subtitle = fmt.Sprintf("%s · %d", item.Subcategory, item.Quantity)
	}
	return item, nil
}

// deriveStatus summarizes how close an item is to its expiry date.
func deriveStatus(expiry sql.NullTime) string {
	if !expiry.Valid {
		return ""
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	e := expiry.Time
	expiryDay := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, time.UTC)
	days := int(expiryDay.Sub(today).Hours() / 24)

	switch {
	case days < 0:
		return "Expired"
	case days == 0:
		return "Use today"
	case days == 1:
		return "1 day left"
	case days <= 7:
		return fmt.Sprintf("%d days left", days)
	default:
		return ""
	}
}

func parseID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}
