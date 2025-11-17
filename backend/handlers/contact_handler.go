package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-aws-backend/models"
	"go-aws-backend/repository"
	"go-aws-backend/service"
)

type ContactHandler struct {
	contactService *service.ContactService
}

func NewContactHandler(contactService *service.ContactService) *ContactHandler {
	return &ContactHandler{
		contactService: contactService,
	}
}

// CreateContact handles POST /api/v1/contacts
func (h *ContactHandler) CreateContact(c *gin.Context) {
	var req models.CreateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	contact, err := h.contactService.CreateContact(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, repository.ErrContactExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Contact already exists",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create contact",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, contact)
}

// GetContact handles GET /api/v1/contacts/:id
func (h *ContactHandler) GetContact(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Contact ID is required",
		})
		return
	}

	contact, err := h.contactService.GetContact(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrContactNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contact not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get contact",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, contact)
}

// UpdateContact handles PUT /api/v1/contacts/:id
func (h *ContactHandler) UpdateContact(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Contact ID is required",
		})
		return
	}

	var req models.UpdateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	contact, err := h.contactService.UpdateContact(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, repository.ErrContactNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contact not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update contact",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, contact)
}

// DeleteContact handles DELETE /api/v1/contacts/:id
func (h *ContactHandler) DeleteContact(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Contact ID is required",
		})
		return
	}

	err := h.contactService.DeleteContact(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrContactNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Contact not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete contact",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Contact deleted successfully",
	})
}

// ListContacts handles GET /api/v1/contacts
func (h *ContactHandler) ListContacts(c *gin.Context) {
	contacts, err := h.contactService.ListContacts(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list contacts",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contacts": contacts,
		"count":    len(contacts),
	})
}

// ListContactsByUser handles GET /api/v1/users/:userId/contacts
func (h *ContactHandler) ListContactsByUser(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID is required",
		})
		return
	}

	contacts, err := h.contactService.ListContactsByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list user contacts",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"contacts": contacts,
		"count":    len(contacts),
	})
}

// ListFavoriteContacts handles GET /api/v1/users/:userId/contacts/favorites
func (h *ContactHandler) ListFavoriteContacts(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "User ID is required",
		})
		return
	}

	contacts, err := h.contactService.ListFavoriteContacts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list favorite contacts",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"contacts": contacts,
		"count":    len(contacts),
	})
}