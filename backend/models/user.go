package models

import "time"


// Main entity with DynamoDB and JSON tags
type User struct {
    ID        string    `json:"id" dynamodbav:"id"`
    Email     string    `json:"email" dynamodbav:"email"`
    FirstName string    `json:"first_name" dynamodbav:"first_name"`
    LastName  string    `json:"last_name" dynamodbav:"last_name"`
    CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
    UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Request DTOs with validation tags
type CreateUserRequest struct {
    Email     string `json:"email" binding:"required,email"`
    FirstName string `json:"first_name" binding:"required"`
    LastName  string `json:"last_name" binding:"required"`
}

type UpdateUserRequest struct {
    Email     string `json:"email" binding:"omitempty,email"`
    FirstName string `json:"first_name" binding:"omitempty"`
    LastName  string `json:"last_name" binding:"omitempty"`
}