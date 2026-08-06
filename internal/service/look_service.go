package service

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"organizing-app-backend/internal/model"
)

var ErrLookNotFound = errors.New("look not found")

// ErrInvalidLook marks a Create argument-validation failure (blank name,
// empty items, or an itemId the requesting user doesn't own), surfaced by
// the controller as 400. Errors returned for a specific case still carry
// their own message but all satisfy errors.Is(err, ErrInvalidLook). Mirrors
// signupValidationError in auth_service.go.
var ErrInvalidLook = errors.New("invalid look")

type lookValidationError struct{ message string }

func (e *lookValidationError) Error() string        { return e.message }
func (e *lookValidationError) Is(target error) bool { return target == ErrInvalidLook }

func newLookValidationError(message string) error {
	return &lookValidationError{message: message}
}

// LookService defines the business operations available on saved looks.
// Every operation is scoped to the authenticated user's id.
type LookService interface {
	List(ctx context.Context, userID int64) ([]model.Look, error)
	Get(ctx context.Context, userID int64, id string) (model.Look, error)
	Create(ctx context.Context, userID int64, look model.Look) (model.Look, error)
	Update(ctx context.Context, userID int64, id string, look model.Look) (model.Look, error)
	Delete(ctx context.Context, userID int64, id string) error
}

// postgresLookService serves looks and their placed items from the
// relational schema (Looks -> LookItems -> Inventories). Item Name/ImageURL
// are derived by joining Inventories, so they're ignored on write.
type postgresLookService struct {
	db *pgxpool.Pool
}

func NewLookService(db *pgxpool.Pool) LookService {
	return &postgresLookService{db: db}
}

const selectLook = `
SELECT id, name, COALESCE(occasion, ''), createdAt, updatedAt
FROM Looks
WHERE userId = $1`

func (s *postgresLookService) List(ctx context.Context, userID int64) ([]model.Look, error) {
	rows, err := s.db.Query(ctx, selectLook+` ORDER BY createdAt DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	looks := make([]model.Look, 0)
	for rows.Next() {
		look, err := scanLook(rows)
		if err != nil {
			return nil, err
		}
		looks = append(looks, look)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.fillItems(ctx, looks); err != nil {
		return nil, err
	}
	return looks, nil
}

func (s *postgresLookService) Get(ctx context.Context, userID int64, id string) (model.Look, error) {
	numericID, err := parseID(id)
	if err != nil {
		return model.Look{}, ErrLookNotFound
	}

	row := s.db.QueryRow(ctx, selectLook+` AND id = $2`, userID, numericID)
	look, err := scanLook(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Look{}, ErrLookNotFound
	}
	if err != nil {
		return model.Look{}, err
	}

	looks := []model.Look{look}
	if err := s.fillItems(ctx, looks); err != nil {
		return model.Look{}, err
	}
	return looks[0], nil
}

// fillItems populates Items on each look from the LookItems junction table,
// joined against Inventories for the response-only Name/ImageURL/CutoutURL
// fields.
func (s *postgresLookService) fillItems(ctx context.Context, looks []model.Look) error {
	byID := make(map[string]*model.Look, len(looks))
	ids := make([]int64, 0, len(looks))
	for i := range looks {
		looks[i].Items = []model.LookItem{}
		byID[looks[i].ID] = &looks[i]
		numericID, err := parseID(looks[i].ID)
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
		`SELECT li.lookId, li.inventoryId, li.x, li.y, li.width, li.height, li.rotation, li.zIndex,
		        i.name, COALESCE(i.imageURL, ''), COALESCE(i.cutoutURL, '')
		 FROM LookItems li
		 JOIN Inventories i ON i.id = li.inventoryId
		 WHERE li.lookId = ANY($1)
		 ORDER BY li.zIndex, li.id`,
		ids,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			lookID      int64
			inventoryID int64
			item        model.LookItem
		)
		if err := rows.Scan(
			&lookID, &inventoryID, &item.X, &item.Y, &item.Width, &item.Height, &item.Rotation, &item.ZIndex,
			&item.Name, &item.ImageURL, &item.CutoutURL,
		); err != nil {
			return err
		}
		item.ItemID = strconv.FormatInt(inventoryID, 10)
		if look := byID[strconv.FormatInt(lookID, 10)]; look != nil {
			look.Items = append(look.Items, item)
		}
	}
	return rows.Err()
}

func (s *postgresLookService) Create(ctx context.Context, userID int64, look model.Look) (model.Look, error) {
	name := strings.TrimSpace(look.Name)
	if name == "" {
		return model.Look{}, newLookValidationError("name is required")
	}
	if len(look.Items) == 0 {
		return model.Look{}, newLookValidationError("at least one item is required")
	}

	itemIDs := make([]int64, len(look.Items))
	for i, item := range look.Items {
		numericID, err := parseID(item.ItemID)
		if err != nil {
			return model.Look{}, newLookValidationError("item not found")
		}
		itemIDs[i] = numericID
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.Look{}, err
	}
	defer tx.Rollback(ctx)

	if err := verifyItemsOwned(ctx, tx, userID, itemIDs); err != nil {
		return model.Look{}, err
	}

	var id int64
	err = tx.QueryRow(
		ctx,
		`INSERT INTO Looks (userId, name, occasion, createdAt, updatedAt)
		 VALUES ($1, $2, NULLIF($3, ''), now(), now())
		 RETURNING id`,
		userID, name, strings.TrimSpace(look.Occasion),
	).Scan(&id)
	if err != nil {
		return model.Look{}, err
	}

	for i, item := range look.Items {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO LookItems (lookId, inventoryId, x, y, width, height, rotation, zIndex)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			id, itemIDs[i], item.X, item.Y, item.Width, item.Height, item.Rotation, item.ZIndex,
		); err != nil {
			return model.Look{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Look{}, err
	}

	return s.Get(ctx, userID, strconv.FormatInt(id, 10))
}

// Update replaces a look's name, occasion, and its entire item set inside a
// single transaction: the row is updated (an affected-rows check of 0 maps
// to ErrLookNotFound, covering both an unknown id and one owned by another
// user, mirroring Delete), then every existing LookItems row for the look is
// deleted and the submitted set re-inserted. Validation mirrors Create
// exactly (trimmed non-empty name, at least one item, every itemId owned by
// userID) and runs before anything is written, and any failure along the way
// rolls back the whole transaction so a look's previous items are never left
// partially wiped.
func (s *postgresLookService) Update(ctx context.Context, userID int64, id string, look model.Look) (model.Look, error) {
	numericID, err := parseID(id)
	if err != nil {
		return model.Look{}, ErrLookNotFound
	}

	name := strings.TrimSpace(look.Name)
	if name == "" {
		return model.Look{}, newLookValidationError("name is required")
	}
	if len(look.Items) == 0 {
		return model.Look{}, newLookValidationError("at least one item is required")
	}

	itemIDs := make([]int64, len(look.Items))
	for i, item := range look.Items {
		numericItemID, err := parseID(item.ItemID)
		if err != nil {
			return model.Look{}, newLookValidationError("item not found")
		}
		itemIDs[i] = numericItemID
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.Look{}, err
	}
	defer tx.Rollback(ctx)

	if err := verifyItemsOwned(ctx, tx, userID, itemIDs); err != nil {
		return model.Look{}, err
	}

	result, err := tx.Exec(
		ctx,
		`UPDATE Looks SET name = $1, occasion = NULLIF($2, ''), updatedAt = now()
		 WHERE id = $3 AND userId = $4`,
		name, strings.TrimSpace(look.Occasion), numericID, userID,
	)
	if err != nil {
		return model.Look{}, err
	}
	if result.RowsAffected() == 0 {
		return model.Look{}, ErrLookNotFound
	}

	if _, err := tx.Exec(ctx, `DELETE FROM LookItems WHERE lookId = $1`, numericID); err != nil {
		return model.Look{}, err
	}

	for i, item := range look.Items {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO LookItems (lookId, inventoryId, x, y, width, height, rotation, zIndex)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			numericID, itemIDs[i], item.X, item.Y, item.Width, item.Height, item.Rotation, item.ZIndex,
		); err != nil {
			return model.Look{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Look{}, err
	}

	return s.Get(ctx, userID, id)
}

// verifyItemsOwned confirms every id in itemIDs is an Inventories row owned
// by userID, returning a lookValidationError (400) otherwise. Unknown ids
// and ids owned by another user are deliberately indistinguishable, so
// nothing about another user's inventory leaks through this endpoint.
func verifyItemsOwned(ctx context.Context, tx pgx.Tx, userID int64, itemIDs []int64) error {
	distinct := make(map[int64]struct{}, len(itemIDs))
	for _, id := range itemIDs {
		distinct[id] = struct{}{}
	}

	rows, err := tx.Query(
		ctx,
		`SELECT id FROM Inventories WHERE userId = $1 AND id = ANY($2)`,
		userID, itemIDs,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	owned := make(map[int64]struct{}, len(distinct))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		owned[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for id := range distinct {
		if _, ok := owned[id]; !ok {
			return newLookValidationError("item not found")
		}
	}
	return nil
}

func (s *postgresLookService) Delete(ctx context.Context, userID int64, id string) error {
	numericID, err := parseID(id)
	if err != nil {
		return ErrLookNotFound
	}

	result, err := s.db.Exec(ctx, `DELETE FROM Looks WHERE id = $1 AND userId = $2`, numericID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrLookNotFound
	}
	return nil
}

func scanLook(row rowScanner) (model.Look, error) {
	var (
		look model.Look
		id   int64
	)
	err := row.Scan(&id, &look.Name, &look.Occasion, &look.CreatedAt, &look.UpdatedAt)
	if err != nil {
		return model.Look{}, err
	}
	look.ID = strconv.FormatInt(id, 10)
	return look, nil
}
