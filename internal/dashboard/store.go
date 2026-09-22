// internal/dashboard/store.go
package dashboard

import (
	"github.com/marceloamoreno87/workflow-dev-template/internal/workflow"
)

// Item is one Work Item projection row.
type Item struct {
	ID      workflow.WorkItemID
	State   workflow.State
	Version workflow.Version
}

// Store holds Work Item projections for the dashboard.
// Task 2 adds Upsert/Get/List.
type Store struct {
	items map[workflow.WorkItemID]Item
}

// NewStore returns an empty projection store.
func NewStore() *Store {
	return &Store{items: map[workflow.WorkItemID]Item{}}
}
