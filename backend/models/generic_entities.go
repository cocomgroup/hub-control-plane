package models

import (
	"fmt"
	"time"
)

// ============================================================================
// Base Types - All models embed this
// ============================================================================

// DynamoDBEntity contains common fields for single-table design
type DynamoDBEntity struct {
	PK        string    `json:"-" dynamodbav:"PK"`           // Partition Key (hidden from JSON)
	SK        string    `json:"-" dynamodbav:"SK"`           // Sort Key (hidden from JSON)
	GSI1PK    string    `json:"-" dynamodbav:"GSI1PK"`       // For querying by entity type
	GSI1SK    string    `json:"-" dynamodbav:"GSI1SK"`       // For sorting within entity type
	EntityType string   `json:"entity_type" dynamodbav:"EntityType"` // USER, CONTACT, ORDER, etc.
	CreatedAt time.Time `json:"created_at" dynamodbav:"CreatedAt"`
	UpdatedAt time.Time `json:"updated_at" dynamodbav:"UpdatedAt"`
}

// GetPK returns the partition key
func (e *DynamoDBEntity) GetPK() string { return e.PK }

// GetSK returns the sort key
func (e *DynamoDBEntity) GetSK() string { return e.SK }

// SetPK sets the partition key
func (e *DynamoDBEntity) SetPK(pk string) { e.PK = pk }

// SetSK sets the sort key
func (e *DynamoDBEntity) SetSK(sk string) { e.SK = sk }

// GetEntityType returns the entity type
func (e *DynamoDBEntity) GetEntityType() string { return e.EntityType }

// SetTimestamps sets created/updated timestamps
func (e *DynamoDBEntity) SetTimestamps() {
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
}

// ============================================================================
// User Model - Single Table Design
// ============================================================================

type UserEntity struct {
	DynamoDBEntity              // Embedded base entity
	ID             string       `json:"id" dynamodbav:"ID"`
	Email          string       `json:"email" dynamodbav:"Email"`
	FirstName      string       `json:"first_name" dynamodbav:"FirstName"`
	LastName       string       `json:"last_name" dynamodbav:"LastName"`
}

// NewUser creates a new user with proper keys
func NewUser(id, email, firstName, lastName string) *UserEntity {
	user := &UserEntity{
		ID:        id,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
	}
	
	// Set single-table design keys
	user.PK = fmt.Sprintf("USER#%s", id)
	user.SK = "METADATA"
	user.GSI1PK = "USER"
	user.GSI1SK = fmt.Sprintf("USER#%s", id)
	user.EntityType = "USER"
	
	return user
}

// ============================================================================
// Contact Model - Single Table Design
// ============================================================================

type ContactEntity struct {
	DynamoDBEntity              // Embedded base entity
	ID             string       `json:"id" dynamodbav:"ID"`
	UserID         string       `json:"user_id" dynamodbav:"UserID"`
	Name           string       `json:"name" dynamodbav:"Name"`
	Email          string       `json:"email" dynamodbav:"Email"`
	Phone          string       `json:"phone" dynamodbav:"Phone"`
	Company        string       `json:"company" dynamodbav:"Company"`
	IsFavorite     bool         `json:"is_favorite" dynamodbav:"IsFavorite"`
}

// NewContact creates a new contact with proper keys
func NewContact(id, userID, name, email, phone, company string, isFavorite bool) *ContactEntity {
	contact := &ContactEntity{
		ID:         id,
		UserID:     userID,
		Name:       name,
		Email:      email,
		Phone:      phone,
		Company:    company,
		IsFavorite: isFavorite,
	}
	
	// Set single-table design keys
	// PK: USER#123 (allows querying all contacts for a user)
	// SK: CONTACT#456 (unique contact identifier)
	contact.PK = fmt.Sprintf("USER#%s", userID)
	contact.SK = fmt.Sprintf("CONTACT#%s", id)
	contact.GSI1PK = "CONTACT"
	contact.GSI1SK = fmt.Sprintf("CONTACT#%s", id)
	contact.EntityType = "CONTACT"
	
	return contact
}


// ============================================================================
// Key Design Patterns Explained
// ============================================================================

/*
SINGLE TABLE DESIGN PATTERNS:

1. USER (standalone entity)
   PK: USER#123
   SK: METADATA
   Access: Direct lookup by user ID

2. CONTACT (belongs to user)
   PK: USER#123
   SK: CONTACT#456
   Access: Query all contacts for a user

3. ORDER (belongs to user, searchable by status)
   PK: USER#123
   SK: ORDER#789
   GSI1SK: ORDER#PENDING#789 (enables filtering by status)
   Access: Query all orders for a user, or filter by status

4. PRODUCT (standalone, searchable by category)
   PK: PRODUCT#111
   SK: METADATA
   GSI1SK: CATEGORY#Electronics#111
   Access: Direct lookup or query by category

5. COMMENT (belongs to post, searchable by user)
   PK: POST#222
   SK: COMMENT#333
   GSI1SK: USER#123#333
   Access: Query all comments for a post, or all comments by a user

GSI1 Usage:
- GSI1PK: Entity type (USER, CONTACT, ORDER, etc.)
- GSI1SK: Custom sorting key for filtering/sorting within type
- Enables queries like:
  * All orders with status "PENDING"
  * All products in category "Electronics"
  * All comments by a specific user

Benefits:
- Single table for all entities
- Efficient queries with proper key design
- Related items stored together (user + contacts)
- Flexible filtering with GSI
- Reduced costs (fewer tables)
*/