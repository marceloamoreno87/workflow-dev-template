// internal/dashboard/store.go
package dashboard

import (
	"sort"
	"sync"

	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

// Item is one Work Item projection row.
type Item struct {
	ID      workflow.WorkItemID
	State   workflow.State
	Version workflow.Version
}

// Store holds Work Item projections for the dashboard.
type Store struct {
	mu    sync.RWMutex
	items map[workflow.WorkItemID]Item
}

// NewStore returns an empty projection store.
func NewStore() *Store {
	return &Store{items: map[workflow.WorkItemID]Item{}}
}

// Upsert inserts or replaces one projection row.
func (s *Store) Upsert(item Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
}

// Get returns one projection row by ID.
func (s *Store) Get(id workflow.WorkItemID) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	return item, ok
}

// List returns every projection row sorted by ID.
func (s *Store) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
