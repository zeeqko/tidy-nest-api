package service

import (
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	"organizing-app-backend/internal/model"
)

var ErrItemNotFound = errors.New("inventory item not found")

// InventoryService defines the business operations available on inventory items.
type InventoryService interface {
	List() ([]model.InventoryItem, error)
	Get(id string) (model.InventoryItem, error)
	Create(item model.InventoryItem) (model.InventoryItem, error)
	Update(id string, item model.InventoryItem) (model.InventoryItem, error)
	Delete(id string) error
}

// inMemoryInventoryService is a stub implementation to be swapped for a
// database-backed implementation later.
type inMemoryInventoryService struct {
	mu    sync.RWMutex
	items map[string]model.InventoryItem
	seq   int
}

func NewInventoryService() InventoryService {
	s := &inMemoryInventoryService{
		items: make(map[string]model.InventoryItem),
	}
	for _, item := range seedItems() {
		s.Create(item)
	}
	return s
}

func (s *inMemoryInventoryService) List() ([]model.InventoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]model.InventoryItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s *inMemoryInventoryService) Get(id string) (model.InventoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[id]
	if !ok {
		return model.InventoryItem{}, ErrItemNotFound
	}
	return item, nil
}

func (s *inMemoryInventoryService) Create(item model.InventoryItem) (model.InventoryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	now := time.Now()
	item.ID = generateID(s.seq)
	item.CreatedAt = now
	item.UpdatedAt = now
	s.items[item.ID] = item
	return item, nil
}

func (s *inMemoryInventoryService) Update(id string, item model.InventoryItem) (model.InventoryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.items[id]
	if !ok {
		return model.InventoryItem{}, ErrItemNotFound
	}

	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	item.UpdatedAt = time.Now()
	s.items[id] = item
	return item, nil
}

func (s *inMemoryInventoryService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrItemNotFound
	}
	delete(s.items, id)
	return nil
}

func generateID(seq int) string {
	return time.Now().UTC().Format("20060102150405") + "-" + strconv.Itoa(seq)
}
