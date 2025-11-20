package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"hub-control-plane/backend/service"
)

type AppHandler struct {
	appService *service.AppServiceWithCache
}

func NewAppHandler(appService *service.AppServiceWithCache) *AppHandler {
	return &AppHandler{
		appService: appService,
	}
}

// ============================================================================
// USER HANDLERS
// ============================================================================

// CreateUser handles POST /api/v1/users
func (h *AppHandler) CreateUser(c *gin.Context) {
	var req struct {
		Email     string `json:"email" binding:"required,email"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.appService.CreateUser(c.Request.Context(), req.Email, req.FirstName, req.LastName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetUser handles GET /api/v1/users/:id
func (h *AppHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")

	user, err := h.appService.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUser handles PUT /api/v1/users/:id
func (h *AppHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.appService.UpdateUser(c.Request.Context(), userID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/users/:id
func (h *AppHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")

	if err := h.appService.DeleteUser(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// ListUsers handles GET /api/v1/users
func (h *AppHandler) ListUsers(c *gin.Context) {
	users, err := h.appService.ListAllUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"users": users, "count": len(users)})
}

// ============================================================================
// CONTACT HANDLERS
// ============================================================================

// CreateContact handles POST /api/v1/users/:userId/contacts
func (h *AppHandler) CreateContact(c *gin.Context) {
	userID := c.Param("userId")
	
	var req struct {
		Name       string `json:"name" binding:"required"`
		Email      string `json:"email"`
		Phone      string `json:"phone"`
		Company    string `json:"company"`
		IsFavorite bool   `json:"is_favorite"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := h.appService.CreateContact(
		c.Request.Context(),
		userID,
		req.Name,
		req.Email,
		req.Phone,
		req.Company,
		req.IsFavorite,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, contact)
}

// GetContact handles GET /api/v1/users/:userId/contacts/:contactId
func (h *AppHandler) GetContact(c *gin.Context) {
	userID := c.Param("userId")
	contactID := c.Param("contactId")

	contact, err := h.appService.GetContact(c.Request.Context(), userID, contactID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contact)
}

// ListUserContacts handles GET /api/v1/users/:userId/contacts
func (h *AppHandler) ListUserContacts(c *gin.Context) {
	userID := c.Param("userId")

	contacts, err := h.appService.ListUserContacts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"contacts": contacts, "count": len(contacts)})
}

// ListFavoriteContacts handles GET /api/v1/users/:userId/contacts/favorites
func (h *AppHandler) ListFavoriteContacts(c *gin.Context) {
	userID := c.Param("userId")

	contacts, err := h.appService.ListFavoriteContacts(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"favorites": contacts, "count": len(contacts)})
}

// UpdateContact handles PUT /api/v1/users/:userId/contacts/:contactId
func (h *AppHandler) UpdateContact(c *gin.Context) {
	userID := c.Param("userId")
	contactID := c.Param("contactId")
	
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact, err := h.appService.UpdateContact(c.Request.Context(), userID, contactID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contact)
}

// DeleteContact handles DELETE /api/v1/users/:userId/contacts/:contactId
func (h *AppHandler) DeleteContact(c *gin.Context) {
	userID := c.Param("userId")
	contactID := c.Param("contactId")

	if err := h.appService.DeleteContact(c.Request.Context(), userID, contactID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Contact deleted successfully"})
}
