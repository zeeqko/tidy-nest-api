package model

import "time"

// TagRef is a tag as carried on an inventory item: clients send just the
// name; responses include the resolved colour.
type TagRef struct {
	Name   string `json:"name"`
	Colour string `json:"colour,omitempty"`
}

// InventoryItem represents a single item being organized/tracked.
type InventoryItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	// CategoryID/SubCategoryID mirror Category.id (string form); nil when the
	// item's subcategory (or its parent category) no longer exists.
	CategoryID    *string  `json:"categoryId"`
	SubCategoryID *string  `json:"subCategoryId"`
	Location      string   `json:"location"`
	Quantity      int      `json:"quantity"`
	Tags          []TagRef `json:"tags"`
	Status        string   `json:"status,omitempty"`
	Notes         string   `json:"notes,omitempty"`
	// URL of the item's photo as served by this backend (e.g. /uploads/ab12.jpg).
	ImageURL string `json:"imageURL,omitempty"`
	// Optional dates in YYYY-MM-DD form; empty means not set.
	ExpiryDate string    `json:"expiryDate,omitempty"`
	OpensOn    string    `json:"opensOn,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
