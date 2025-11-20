package repository

import (
	"context"
	"hub-control-plane/backend/models"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, id string, updates map[string]interface{}) (*models.User, error)
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]*models.User, error)
}

// ContactRepository defines the interface for contact data operations
type ContactRepository interface {
	CreateContact(ctx context.Context, contact *models.Contact) error
	GetContact(ctx context.Context, id string) (*models.Contact, error)
	UpdateContact(ctx context.Context, id string, updates map[string]interface{}) (*models.Contact, error)
	DeleteContact(ctx context.Context, id string) error
	ListContacts(ctx context.Context) ([]*models.Contact, error)
	ListContactsByUser(ctx context.Context, userID string) ([]*models.Contact, error)
	ListFavoriteContacts(ctx context.Context, userID string) ([]*models.Contact, error)
}

// UserCache defines the interface for user caching operations
type UserCache interface {
	GetUser(ctx context.Context, id string) (*models.User, error)
	SetUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, id string) error
	InvalidateUserList(ctx context.Context) error
	GetUserList(ctx context.Context) ([]*models.User, error)
	SetUserList(ctx context.Context, users []*models.User) error
}

// ContactCache defines the interface for contact caching operations
type ContactCache interface {
	GetContact(ctx context.Context, id string) (*models.Contact, error)
	SetContact(ctx context.Context, contact *models.Contact) error
	DeleteContact(ctx context.Context, id string) error
	InvalidateContactList(ctx context.Context) error
	InvalidateUserContactList(ctx context.Context, userID string) error
	InvalidateFavoriteContactList(ctx context.Context, userID string) error
	GetContactList(ctx context.Context) ([]*models.Contact, error)
	SetContactList(ctx context.Context, contacts []*models.Contact) error
	GetUserContactList(ctx context.Context, userID string) ([]*models.Contact, error)
	SetUserContactList(ctx context.Context, userID string, contacts []*models.Contact) error
	GetFavoriteContactList(ctx context.Context, userID string) ([]*models.Contact, error)
	SetFavoriteContactList(ctx context.Context, userID string, contacts []*models.Contact) error
}