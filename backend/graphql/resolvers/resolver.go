package resolvers

import (
	"context"

	// Local packages
	"hub-control-plane/backend/models"
	"hub-control-plane/backend/service"
	"hub-control-plane/backend/graphql"
)

// Resolver is the root resolver
type Resolver struct {
	appService *service.AppServiceWithCache
}

// NewResolver creates a new GraphQL resolver
func NewResolver(appService *service.AppServiceWithCache) *Resolver {
	return &Resolver{
		appService: appService,
	}
}

// ============================================================================
// QUERY RESOLVERS
// ============================================================================

// Users resolves the users list query
func (r *Resolver) Users(ctx context.Context, limit *int, offset *int) ([]*models.UserEntity, error) {
	// For now, return all users (you can add pagination later)
	return r.appService.ListAllUsers(ctx)
}

// Contacts resolves the contacts list query
func (r *Resolver) Contacts(ctx context.Context, limit *int, offset *int) ([]*models.ContactEntity, error) {
	return r.appService.ListAllContacts(ctx)
}

// UserContacts resolves contacts for a specific user
func (r *Resolver) UserContacts(ctx context.Context, userID string, favorites *bool) ([]*models.ContactEntity, error) {
	if favorites != nil && *favorites {
		return r.appService.ListFavoriteContacts(ctx, userID)
	}
	return r.appService.ListUserContacts(ctx, userID)
}

// ============================================================================
// MUTATION RESOLVERS
// ============================================================================

// CreateUser resolves the createUser mutation
func (r *Resolver) CreateUser(ctx context.Context, input graphql.CreateUserInput) (*models.UserEntity, error) {
	return r.appService.CreateUser(ctx, input.Email, input.FirstName, input.LastName)
}

// UpdateUser resolves the updateUser mutation
func (r *Resolver) UpdateUser(ctx context.Context, id string, input graphql.UpdateUserInput) (*models.UserEntity, error) {
	updates := make(map[string]interface{})
	
	if input.Email != nil {
		updates["Email"] = *input.Email
	}
	if input.FirstName != nil {
		updates["FirstName"] = *input.FirstName
	}
	if input.LastName != nil {
		updates["LastName"] = *input.LastName
	}
	
	return r.appService.UpdateUser(ctx, id, updates)
}

// DeleteUser resolves the deleteUser mutation
func (r *Resolver) DeleteUser(ctx context.Context, id string) (bool, error) {
	err := r.appService.DeleteUser(ctx, id)
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateContact resolves the createContact mutation
func (r *Resolver) CreateContact(ctx context.Context, input graphql.CreateContactInput) (*models.ContactEntity, error) {
	email := ""
	if input.Email != nil {
		email = *input.Email
	}
	
	phone := ""
	if input.Phone != nil {
		phone = *input.Phone
	}
	
	company := ""
	if input.Company != nil {
		company = *input.Company
	}
	
	isFavorite := false
	if input.IsFavorite != nil {
		isFavorite = *input.IsFavorite
	}
	
	return r.appService.CreateContact(ctx, input.UserID, input.Name, email, phone, company, isFavorite)
}

// UpdateContact resolves the updateContact mutation
func (r *Resolver) UpdateContact(ctx context.Context, id string, userID string, input graphql.UpdateContactInput) (*models.ContactEntity, error) {
	updates := make(map[string]interface{})
	
	if input.Name != nil {
		updates["Name"] = *input.Name
	}
	if input.Email != nil {
		updates["Email"] = *input.Email
	}
	if input.Phone != nil {
		updates["Phone"] = *input.Phone
	}
	if input.Company != nil {
		updates["Company"] = *input.Company
	}
	if input.IsFavorite != nil {
		updates["IsFavorite"] = *input.IsFavorite
	}
	if input.Tags != nil {
		updates["Tags"] = input.Tags
	}
	
	return r.appService.UpdateContact(ctx, userID, id, updates)
}

// DeleteContact resolves the deleteContact mutation
func (r *Resolver) DeleteContact(ctx context.Context, id string, userID string) (bool, error) {
	err := r.appService.DeleteContact(ctx, userID, id)
	if err != nil {
		return false, err
	}
	return true, nil
}

// ============================================================================
// FIELD RESOLVERS (for nested queries)
// ============================================================================

// UserResolver handles User type field resolution
type UserResolver struct {
	*Resolver
}

// Contacts resolves the contacts field on User
func (r *UserResolver) Contacts(ctx context.Context, obj *models.UserEntity, limit *int, favorites *bool) ([]*models.ContactEntity, error) {
	if favorites != nil && *favorites {
		return r.appService.ListFavoriteContacts(ctx, obj.ID)
	}
	return r.appService.ListUserContacts(ctx, obj.ID)
}
