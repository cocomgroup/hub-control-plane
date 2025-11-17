package models

import "time"

// Main entity with DynamoDB and JSON tags
type Contact struct {
    ID        string    `json:"id" dynamodbav:"id"`
	UserID    string    `json:"email" dynamodbav:"userid"`
	Name      string    `json:"name" dynamodbav:"name"`
    Email     string    `json:"email" dynamodbav:"email"`
    Phone     string    `json:"phone" dynamodbav:"phone"`
    Company   string    `json:"company" dynamodbav:"company"`
	JobTitle  string    `json:"job_title" dynamodbav:"job_title"`
	Address   string    `json:"address" dynamodbav:"address"`
	Notes     string    `json:"notes" dynamodbav:"notes"`
	IsFavorite bool     `json:"is_favorite" dynamodbav:"is_favorite"`
	Tags      []string  `json:"tags" dynamodbav:"tags"`
    CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
    UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Request DTOs with validation tags
type CreateContactRequest struct {
	UserID	  string `json:"userid" binding:"required,userid"`
	Name      string `json:"name" binding:"required"`
    Email     string `json:"email" binding:"required,email"`
    Phone     string `json:"phone" binding:"required"`
    Company   string `json:"company" binding:"required"`
	JobTitle  string `json:"jobtitle" binding:"required"`
	Address   string `json:"address" binding:"required"`
	Notes     string `json:"notes" binding:"required"`
	IsFavorite bool   `json:"is_favorite" binding:"required"`
	Tags      []string `json:"tags" binding:"required"`
	CreatedAt time.Time `json:"created_at" binding:"required"`
}

type UpdateContactRequest struct {
	UserID	  string `json:"userid" binding:"required,userid"`
	Name      string `json:"name" binding:"omitempty"`
    Email     string `json:"email" binding:"omitempty,email"`
	Phone     string `json:"phone" binding:"omitempty"`
	Company   string `json:"company" binding:"omitempty"`
	JobTitle  string `json:"jobtitle" binding:"omitempty"`
	Address   string `json:"address" binding:"omitempty"`
	Notes     string `json:"notes" binding:"omitempty"`
	IsFavorite *bool   `json:"is_favorite" binding:"omitempty"`
	Tags      []string `json:"tags" binding:"omitempty"`
	UpdatedAt time.Time `json:"updated_at" binding:"required,updated_at"`
}