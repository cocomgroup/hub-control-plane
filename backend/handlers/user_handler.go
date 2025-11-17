package handlers

type UserHandler struct {
    userService *service.UserService
}

// POST /api/v1/users - Create user
func (h *UserHandler) CreateUser(c *gin.Context) {
    var req models.CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request body"})
        return
    }
    
    user, err := h.userService.CreateUser(c.Request.Context(), &req)
    if errors.Is(err, repository.ErrUserExists) {
        c.JSON(409, gin.H{"error": "User already exists"})
        return
    }
    c.JSON(201, user)
}

// GET /api/v1/users/:id - Get user
func (h *UserHandler) GetUser(c *gin.Context)

// PUT /api/v1/users/:id - Update user
func (h *UserHandler) UpdateUser(c *gin.Context)

// DELETE /api/v1/users/:id - Delete user
func (h *UserHandler) DeleteUser(c *gin.Context)

// GET /api/v1/users - List all users
func (h *UserHandler) ListUsers(c *gin.Context)