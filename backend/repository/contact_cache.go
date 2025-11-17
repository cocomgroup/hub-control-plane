package repository

import (
	"errors"
	"sync"
	"time"
)

// Contact represents a minimal contact record stored in the cache.
type Contact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ContactCache is a simple in-memory thread-safe cache for Contact records.
type ContactCache struct {
	mu    sync.RWMutex
	items map[string]Contact
}

// NewContactCache creates an initialized ContactCache.
func NewContactCache() *ContactCache {
	return &ContactCache{
		items: make(map[string]Contact),
	}
}

// Get returns a contact by id. The boolean indicates whether the contact was found.
func (c *ContactCache) Get(id string) (Contact, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	contact, ok := c.items[id]
	return contact, ok
}

// GetAll returns a slice with all contacts currently in the cache.
func (c *ContactCache) GetAll() []Contact {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Contact, 0, len(c.items))
	for _, v := range c.items {
		out = append(out, v)
	}
	return out
}

// Put inserts or replaces a contact in the cache. UpdatedAt is set to now.
func (c *ContactCache) Put(contact Contact) {
	c.mu.Lock()
	defer c.mu.Unlock()
	contact.UpdatedAt = time.Now().UTC()
	c.items[contact.ID] = contact
}

// Update applies an updater function to an existing contact.
// Returns an error if the contact does not exist.
func (c *ContactCache) Update(id string, updater func(Contact) Contact) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.items[id]
	if !ok {
		return errors.New("contact not found")
	}
	updated := updater(current)
	updated.UpdatedAt = time.Now().UTC()
	c.items[id] = updated
	return nil
}

// Delete removes a contact from the cache. It is a no-op if the id doesn't exist.
func (c *ContactCache) Delete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, id)
}

// Clear removes all contacts from the cache.
func (c *ContactCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]Contact)
}

// Size returns the number of contacts in the cache.
func (c *ContactCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}