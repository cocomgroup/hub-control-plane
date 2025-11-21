package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"hub-control-plane/backend/models"
	"hub-control-plane/backend/repository"
)

// AppServiceWithCache provides business logic with integrated caching
type AppServiceWithCache struct {
	repo  *repository.GenericRepository
	cache *redis.Client
	ttl   time.Duration
}

// NewAppServiceWithCache creates a new application service with caching
func NewAppServiceWithCache(repo *repository.GenericRepository, cache *redis.Client) *AppServiceWithCache {
	return &AppServiceWithCache{
		repo:  repo,
		cache: cache,
		ttl:   5 * time.Minute, // Default cache TTL
	}
}

// ============================================================================
// USER OPERATIONS WITH CACHING
// ============================================================================

// CreateUser creates a new user
// Flow: Save to DB → Cache individual → Invalidate list cache
func (s *AppServiceWithCache) CreateUser(ctx context.Context, email, firstName, lastName string) (*models.UserEntity, error) {
	userID := uuid.New().String()
	user := models.NewUser(userID, email, firstName, lastName)

	// 1. Save to DynamoDB
	if err := s.repo.PutIfNotExists(ctx, user); err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, errors.New("user already exists")
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 2. Cache the individual user
	if err := s.cacheUser(ctx, user); err != nil {
		log.Printf("Warning: failed to cache user: %v", err)
	}

	// 3. Invalidate the user list cache
	if err := s.invalidateUserListCache(ctx); err != nil {
		log.Printf("Warning: failed to invalidate user list cache: %v", err)
	}

	log.Printf("Created user: %s (%s)", userID, email)
	return user, nil
}

// GetUser retrieves a user by ID with caching
// Flow: Check cache → If miss, get from DB → Cache it → Return
func (s *AppServiceWithCache) GetUser(ctx context.Context, userID string) (*models.UserEntity, error) {
	cacheKey := fmt.Sprintf("user:%s", userID)

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for user: %s", userID)
		var user models.UserEntity
		if err := json.Unmarshal([]byte(cached), &user); err == nil {
			return &user, nil
		}
	}

	// 2. Cache MISS - get from DynamoDB
	log.Printf("Cache MISS for user: %s", userID)
	user := &models.UserEntity{}
	pk := fmt.Sprintf("USER#%s", userID)
	sk := "METADATA"

	if err := s.repo.Get(ctx, pk, sk, user); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 3. Cache the result
	if err := s.cacheUser(ctx, user); err != nil {
		log.Printf("Warning: failed to cache user: %v", err)
	}

	return user, nil
}

// UpdateUser updates user information
// Flow: Update in DB → Update cache → Invalidate list cache
func (s *AppServiceWithCache) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*models.UserEntity, error) {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := "METADATA"

	// 1. Update in DynamoDB
	if err := s.repo.Update(ctx, pk, sk, updates); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// 2. Get the updated user
	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. Update cache (GetUser already cached it, but let's be explicit)
	if err := s.cacheUser(ctx, user); err != nil {
		log.Printf("Warning: failed to update cache: %v", err)
	}

	// 4. Invalidate the user list cache
	if err := s.invalidateUserListCache(ctx); err != nil {
		log.Printf("Warning: failed to invalidate user list cache: %v", err)
	}

	log.Printf("Updated user: %s", userID)
	return user, nil
}

// DeleteUser deletes a user
// Flow: Delete from DB → Delete from cache → Invalidate list cache
func (s *AppServiceWithCache) DeleteUser(ctx context.Context, userID string) error {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := "METADATA"

	// 1. Delete from DynamoDB
	if err := s.repo.Delete(ctx, pk, sk); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errors.New("user not found")
		}
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// 2. Delete from cache
	cacheKey := fmt.Sprintf("user:%s", userID)
	if err := s.cache.Del(ctx, cacheKey).Err(); err != nil {
		log.Printf("Warning: failed to delete from cache: %v", err)
	}

	// 3. Invalidate the user list cache
	if err := s.invalidateUserListCache(ctx); err != nil {
		log.Printf("Warning: failed to invalidate user list cache: %v", err)
	}

	log.Printf("Deleted user: %s", userID)
	return nil
}

// ListAllUsers returns all users with list caching
// Flow: Check list cache → If miss, query DB → Cache list → Return
func (s *AppServiceWithCache) ListAllUsers(ctx context.Context) ([]*models.UserEntity, error) {
	cacheKey := "users:list"

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for user list")
		var users []*models.UserEntity
		if err := json.Unmarshal([]byte(cached), &users); err == nil {
			return users, nil
		}
	}

	// 2. Cache MISS - query DynamoDB
	log.Printf("Cache MISS for user list")
	var users []*models.UserEntity
	if err := s.repo.QueryByEntityType(ctx, "USER", &users); err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// 3. Cache the list
	if data, err := json.Marshal(users); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data, s.ttl).Err(); err != nil {
			log.Printf("Warning: failed to cache user list: %v", err)
		}
	}

	return users, nil
}

// ============================================================================
// CONTACT OPERATIONS WITH CACHING
// ============================================================================

// CreateContact creates a new contact for a user
// Flow: Save to DB → Cache individual → Invalidate user's contact list cache
func (s *AppServiceWithCache) CreateContact(ctx context.Context, userID, name, email, phone, company string, isFavorite bool) (*models.ContactEntity, error) {
	contactID := uuid.New().String()
	contact := models.NewContact(contactID, userID, name, email, phone, company, isFavorite)

	// 1. Save to DynamoDB
	if err := s.repo.Put(ctx, contact); err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	// 2. Cache the individual contact
	if err := s.cacheContact(ctx, contact); err != nil {
		log.Printf("Warning: failed to cache contact: %v", err)
	}

	// 3. Invalidate user's contact list caches
	if err := s.invalidateUserContactCaches(ctx, userID); err != nil {
		log.Printf("Warning: failed to invalidate contact caches: %v", err)
	}

	log.Printf("Created contact: %s for user: %s", contactID, userID)
	return contact, nil
}

// GetContact retrieves a specific contact with caching
// Flow: Check cache → If miss, get from DB → Cache it → Return
func (s *AppServiceWithCache) GetContact(ctx context.Context, userID, contactID string) (*models.ContactEntity, error) {
	cacheKey := fmt.Sprintf("contact:%s:%s", userID, contactID)

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for contact: %s", contactID)
		var contact models.ContactEntity
		if err := json.Unmarshal([]byte(cached), &contact); err == nil {
			return &contact, nil
		}
	}

	// 2. Cache MISS - get from DynamoDB
	log.Printf("Cache MISS for contact: %s", contactID)
	contact := &models.ContactEntity{}
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("CONTACT#%s", contactID)

	if err := s.repo.Get(ctx, pk, sk, contact); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errors.New("contact not found")
		}
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}

	// 3. Cache the result
	if err := s.cacheContact(ctx, contact); err != nil {
		log.Printf("Warning: failed to cache contact: %v", err)
	}

	return contact, nil
}

// ListUserContacts returns all contacts for a user with caching
// Flow: Check cache → If miss, query DB → Cache list → Return
func (s *AppServiceWithCache) ListUserContacts(ctx context.Context, userID string) ([]*models.ContactEntity, error) {
	cacheKey := fmt.Sprintf("contacts:user:%s", userID)

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for user %s contacts", userID)
		var contacts []*models.ContactEntity
		if err := json.Unmarshal([]byte(cached), &contacts); err == nil {
			return contacts, nil
		}
	}

	// 2. Cache MISS - query DynamoDB
	log.Printf("Cache MISS for user %s contacts", userID)
	var contacts []*models.ContactEntity
	pk := fmt.Sprintf("USER#%s", userID)

	if err := s.repo.Query(ctx, pk, "CONTACT#", &contacts); err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}

	// 3. Cache the list
	if data, err := json.Marshal(contacts); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data, s.ttl).Err(); err != nil {
			log.Printf("Warning: failed to cache contact list: %v", err)
		}
	}

	return contacts, nil
}

// ListFavoriteContacts returns only favorite contacts for a user with caching
// Flow: Check cache → If miss, query DB with filter → Cache list → Return
func (s *AppServiceWithCache) ListFavoriteContacts(ctx context.Context, userID string) ([]*models.ContactEntity, error) {
	cacheKey := fmt.Sprintf("contacts:favorites:%s", userID)

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for user %s favorites", userID)
		var contacts []*models.ContactEntity
		if err := json.Unmarshal([]byte(cached), &contacts); err == nil {
			return contacts, nil
		}
	}

	// 2. Cache MISS - query DynamoDB with filter
	log.Printf("Cache MISS for user %s favorites", userID)
	var contacts []*models.ContactEntity
	pk := fmt.Sprintf("USER#%s", userID)
	filter := expression.Name("IsFavorite").Equal(expression.Value(true))

	if err := s.repo.QueryWithFilter(ctx, pk, "CONTACT#", filter, &contacts); err != nil {
		return nil, fmt.Errorf("failed to list favorite contacts: %w", err)
	}

	// 3. Cache the list
	if data, err := json.Marshal(contacts); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data, s.ttl).Err(); err != nil {
			log.Printf("Warning: failed to cache favorites: %v", err)
		}
	}

	return contacts, nil
}

// UpdateContact updates contact information
// Flow: Update in DB → Update cache → Invalidate list caches
func (s *AppServiceWithCache) UpdateContact(ctx context.Context, userID, contactID string, updates map[string]interface{}) (*models.ContactEntity, error) {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("CONTACT#%s", contactID)

	// 1. Update in DynamoDB
	if err := s.repo.Update(ctx, pk, sk, updates); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errors.New("contact not found")
		}
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}

	// 2. Get the updated contact
	contact, err := s.GetContact(ctx, userID, contactID)
	if err != nil {
		return nil, err
	}

	// 3. Update cache (GetContact already cached it)
	if err := s.cacheContact(ctx, contact); err != nil {
		log.Printf("Warning: failed to update cache: %v", err)
	}

	// 4. Invalidate list caches
	if err := s.invalidateUserContactCaches(ctx, userID); err != nil {
		log.Printf("Warning: failed to invalidate contact caches: %v", err)
	}

	log.Printf("Updated contact: %s for user: %s", contactID, userID)
	return contact, nil
}

// DeleteContact deletes a contact
// Flow: Delete from DB → Delete from cache → Invalidate list caches
func (s *AppServiceWithCache) DeleteContact(ctx context.Context, userID, contactID string) error {
	pk := fmt.Sprintf("USER#%s", userID)
	sk := fmt.Sprintf("CONTACT#%s", contactID)

	// 1. Delete from DynamoDB
	if err := s.repo.Delete(ctx, pk, sk); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errors.New("contact not found")
		}
		return fmt.Errorf("failed to delete contact: %w", err)
	}

	// 2. Delete from cache
	cacheKey := fmt.Sprintf("contact:%s:%s", userID, contactID)
	if err := s.cache.Del(ctx, cacheKey).Err(); err != nil {
		log.Printf("Warning: failed to delete from cache: %v", err)
	}

	// 3. Invalidate list caches
	if err := s.invalidateUserContactCaches(ctx, userID); err != nil {
		log.Printf("Warning: failed to invalidate contact caches: %v", err)
	}

	log.Printf("Deleted contact: %s for user: %s", contactID, userID)
	return nil
}

// ListAllUsers returns all users with list caching
// Flow: Check list cache → If miss, query DB → Cache list → Return
func (s *AppServiceWithCache) ListAllContacts(ctx context.Context) ([]*models.ContactEntity, error) {
	cacheKey := "contacts:list"

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for contact list")
		var users []*models.ContactEntity
		if err := json.Unmarshal([]byte(cached), &users); err == nil {
			return users, nil
		}
	}

	// 2. Cache MISS - query DynamoDB
	log.Printf("Cache MISS for contact list")
	var contacts []*models.ContactEntity
	if err := s.repo.QueryByEntityType(ctx, "CONTACT", &contacts); err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}

	// 3. Cache the list
	if data, err := json.Marshal(contacts); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data, s.ttl).Err(); err != nil {
			log.Printf("Warning: failed to cache contact list: %v", err)
		}
	}

	return contacts, nil
}

// ============================================================================
// CACHE HELPER METHODS
// ============================================================================

// cacheUser caches an individual user
func (s *AppServiceWithCache) cacheUser(ctx context.Context, user *models.UserEntity) error {
	cacheKey := fmt.Sprintf("user:%s", user.ID)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, cacheKey, data, s.ttl).Err()
}

// invalidateUserListCache invalidates the user list cache
func (s *AppServiceWithCache) invalidateUserListCache(ctx context.Context) error {
	return s.cache.Del(ctx, "users:list").Err()
}

// cacheContact caches an individual contact
func (s *AppServiceWithCache) cacheContact(ctx context.Context, contact *models.ContactEntity) error {
	cacheKey := fmt.Sprintf("contact:%s:%s", contact.UserID, contact.ID)
	data, err := json.Marshal(contact)
	if err != nil {
		return err
	}
	return s.cache.Set(ctx, cacheKey, data, s.ttl).Err()
}

// invalidateUserContactCaches invalidates all contact caches for a user
func (s *AppServiceWithCache) invalidateUserContactCaches(ctx context.Context, userID string) error {
	// Invalidate user's contact list
	if err := s.cache.Del(ctx, fmt.Sprintf("contacts:user:%s", userID)).Err(); err != nil {
		return err
	}
	
	// Invalidate user's favorites list
	if err := s.cache.Del(ctx, fmt.Sprintf("contacts:favorites:%s", userID)).Err(); err != nil {
		return err
	}
	
	return nil
}

// ============================================================================
// DASHBOARD WITH CACHING
// ============================================================================

// GetUserDashboard gets all data for a user with caching
// Flow: Check cache → If miss, query DB → Cache dashboard → Return
func (s *AppServiceWithCache) GetUserDashboard(ctx context.Context, userID string) (*UserDashboard, error) {
	cacheKey := fmt.Sprintf("dashboard:%s", userID)

	// 1. Try to get from cache
	cached, err := s.cache.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache HIT!
		log.Printf("Cache HIT for user %s dashboard", userID)
		var dashboard UserDashboard
		if err := json.Unmarshal([]byte(cached), &dashboard); err == nil {
			return &dashboard, nil
		}
	}

	// 2. Cache MISS - query DynamoDB
	log.Printf("Cache MISS for user %s dashboard", userID)
	pk := fmt.Sprintf("USER#%s", userID)
	
	var allItems []map[string]interface{}
	if err := s.repo.Query(ctx, pk, "", &allItems); err != nil {
		return nil, fmt.Errorf("failed to get user dashboard: %w", err)
	}

	dashboard := &UserDashboard{
		Contacts: make([]*models.ContactEntity, 0),
		//Orders:   make([]*models.OrderEntity, 0),
	}

	// Separate items by entity type
	for _, item := range allItems {
		entityType, _ := item["EntityType"].(string)
		
		switch entityType {
		case "USER":
			user := &models.UserEntity{}
			dashboard.User = user
		case "CONTACT":
			contact := &models.ContactEntity{}
			dashboard.Contacts = append(dashboard.Contacts, contact)
		//case "ORDER":
		//	order := &models.OrderEntity{}
		//	dashboard.Orders = append(dashboard.Orders, order)
		}
	}

	// 3. Cache the dashboard
	if data, err := json.Marshal(dashboard); err == nil {
		// Shorter TTL for dashboard since it aggregates multiple entities
		if err := s.cache.Set(ctx, cacheKey, data, 2*time.Minute).Err(); err != nil {
			log.Printf("Warning: failed to cache dashboard: %v", err)
		}
	}

	return dashboard, nil
}

// ============================================================================
// HELPER TYPES
// ============================================================================

type UserDashboard struct {
	User     *models.UserEntity        `json:"user"`
	Contacts []*models.ContactEntity   `json:"contacts"`
	//Orders   []*models.OrderEntity     `json:"orders"`
}