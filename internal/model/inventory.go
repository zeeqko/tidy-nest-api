package model

import "time"

// InventoryItem represents a single item being organized/tracked.
type InventoryItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Subtitle    string    `json:"subtitle"`
	Category    string    `json:"category"`
	Subcategory string    `json:"subcategory"`
	Location    string    `json:"location"`
	Quantity    int       `json:"quantity"`
	Tag         string    `json:"tag,omitempty"`
	Status      string    `json:"status,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
