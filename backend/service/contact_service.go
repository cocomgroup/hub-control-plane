package service

import (
	"context"
	"log"

	"github.com/google/uuid"
	"go-aws-backend/models"
	"go-aws-backend/repository"
)

type ContactService struct {
	contactRepo *repository.ContactDynamoDBRepository
	cache       *repository.ContactRedisCache
}

func NewContactService(contactRepo *repository.ContactDynamoDBRepository, cache *repository.ContactRedisCache) *ContactService {
	return &ContactService{
		contactRepo: contactRepo,
		cache:       cache,
	}
}

// CreateContact creates a new contact
func (s *ContactService) CreateContact(ctx context.Context, req *models.CreateContactRequest) (*models.Contact, error) {
	contact := &models.Contact{
		ID:         uuid.New().String(),
		UserID:     req.UserID,
		Name:       req.Name,
		Email:      req.Email,
		Phone:      req.Phone,
		Company:    req.Company,
		JobTitle:   req.JobTitle,
		Address:    req.Address,
		Notes:      req.Notes,
		IsFavorite: req.IsFavorite,
		Tags:       req.Tags,
	}

	if contact.Tags == nil {
		contact.Tags = []string{}
	}

	if err := s.contactRepo.CreateContact(ctx, contact); err != nil {
		return nil, err
	}

	// Cache the new contact
	if err := s.cache.SetContact(ctx, contact); err != nil {
		log.Printf("Failed to cache contact: %v", err)
	}

	// Invalidate list caches
	s.invalidateCachesForUser(ctx, contact.UserID)

	return contact, nil
}

// GetContact retrieves a contact by ID (with caching)
func (s *ContactService) GetContact(ctx context.Context, id string) (*models.Contact, error) {
	// Try to get from cache first
	contact, err := s.cache.GetContact(ctx, id)
	if err != nil {
		log.Printf("Cache error: %v", err)
	}
	
	if contact != nil {
		log.Printf("Cache hit for contact: %s", id)
		return contact, nil
	}

	// Cache miss - get from DynamoDB
	log.Printf("Cache miss for contact: %s", id)
	contact, err = s.contactRepo.GetContact(ctx, id)
	if err != nil {
		return nil, err
	}

	// Store in cache for next time
	if err := s.cache.SetContact(ctx, contact); err != nil {
		log.Printf("Failed to cache contact: %v", err)
	}

	return contact, nil
}

// UpdateContact updates an existing contact
func (s *ContactService) UpdateContact(ctx context.Context, id string, req *models.UpdateContactRequest) (*models.Contact, error) {
	updates := make(map[string]interface{})
	
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.Company != "" {
		updates["company"] = req.Company
	}
	if req.JobTitle != "" {
		updates["job_title"] = req.JobTitle
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.Notes != "" {
		updates["notes"] = req.Notes
	}
	if req.IsFavorite != nil {
		updates["is_favorite"] = *req.IsFavorite
	}
	if req.Tags != nil {
		updates["tags"] = req.Tags
	}

	if len(updates) == 0 {
		// No updates provided, just return the existing contact
		return s.GetContact(ctx, id)
	}

	contact, err := s.contactRepo.UpdateContact(ctx, id, updates)
	if err != nil {
		return nil, err
	}

	// Update cache
	if err := s.cache.SetContact(ctx, contact); err != nil {
		log.Printf("Failed to update cache: %v", err)
	}

	// Invalidate list caches
	s.invalidateCachesForUser(ctx, contact.UserID)

	return contact, nil
}

// DeleteContact deletes a contact
func (s *ContactService) DeleteContact(ctx context.Context, id string) error {
	// Get contact first to know which user's cache to invalidate
	contact, err := s.contactRepo.GetContact(ctx, id)
	if err != nil {
		return err
	}

	if err := s.contactRepo.DeleteContact(ctx, id); err != nil {
		return err
	}

	// Remove from cache
	if err := s.cache.DeleteContact(ctx, id); err != nil {
		log.Printf("Failed to delete from cache: %v", err)
	}

	// Invalidate list caches
	s.invalidateCachesForUser(ctx, contact.UserID)

	return nil
}

// ListContacts returns all contacts (with caching)
func (s *ContactService) ListContacts(ctx context.Context) ([]*models.Contact, error) {
	// Try to get from cache first
	contacts, err := s.cache.GetContactList(ctx)
	if err != nil {
		log.Printf("Cache error: %v", err)
	}
	
	if contacts != nil {
		log.Printf("Cache hit for contact list")
		return contacts, nil
	}

	// Cache miss - get from DynamoDB
	log.Printf("Cache miss for contact list")
	contacts, err = s.contactRepo.ListContacts(ctx)
	if err != nil {
		return nil, err
	}

	// Store in cache for next time
	if err := s.cache.SetContactList(ctx, contacts); err != nil {
		log.Printf("Failed to cache contact list: %v", err)
	}

	return contacts, nil
}

// ListContactsByUser returns all contacts for a specific user (with caching)
func (s *ContactService) ListContactsByUser(ctx context.Context, userID string) ([]*models.Contact, error) {
	// Try to get from cache first
	contacts, err := s.cache.GetUserContactList(ctx, userID)
	if err != nil {
		log.Printf("Cache error: %v", err)
	}
	
	if contacts != nil {
		log.Printf("Cache hit for user %s contacts", userID)
		return contacts, nil
	}

	// Cache miss - get from DynamoDB
	log.Printf("Cache miss for user %s contacts", userID)
	contacts, err = s.contactRepo.ListContactsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Store in cache for next time
	if err := s.cache.SetUserContactList(ctx, userID, contacts); err != nil {
		log.Printf("Failed to cache user contacts: %v", err)
	}

	return contacts, nil
}

// ListFavoriteContacts returns favorite contacts for a user (with caching)
func (s *ContactService) ListFavoriteContacts(ctx context.Context, userID string) ([]*models.Contact, error) {
	// Try to get from cache first
	contacts, err := s.cache.GetFavoriteContactList(ctx, userID)
	if err != nil {
		log.Printf("Cache error: %v", err)
	}
	
	if contacts != nil {
		log.Printf("Cache hit for user %s favorites", userID)
		return contacts, nil
	}

	// Cache miss - get from DynamoDB
	log.Printf("Cache miss for user %s favorites", userID)
	contacts, err = s.contactRepo.ListFavoriteContacts(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Store in cache for next time
	if err := s.cache.SetFavoriteContactList(ctx, userID, contacts); err != nil {
		log.Printf("Failed to cache favorite contacts: %v", err)
	}

	return contacts, nil
}

// invalidateCachesForUser invalidates all contact-related caches for a user
func (s *ContactService) invalidateCachesForUser(ctx context.Context, userID string) {
	if err := s.cache.InvalidateContactList(ctx); err != nil {
		log.Printf("Failed to invalidate contact list cache: %v", err)
	}
	
	if err := s.cache.InvalidateUserContactList(ctx, userID); err != nil {
		log.Printf("Failed to invalidate user contact list cache: %v", err)
	}
	
	if err := s.cache.InvalidateFavoriteContactList(ctx, userID); err != nil {
		log.Printf("Failed to invalidate favorite contact list cache: %v", err)
	}
}