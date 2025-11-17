package service

type UserService struct {
    dynamoRepo *repository.DynamoDBRepository
    cache      *repository.RedisCache
}

// Create: Save to DB → Cache → Invalidate list
func (s *UserService) CreateUser(ctx, req) (*models.User, error) {
    user := &models.User{ID: uuid.New().String(), 
        ...}
    s.dynamoRepo.CreateUser(ctx, user)
    s.cache.SetUser(ctx, user)
    s.cache.InvalidateUserList(ctx)
}

// Get: Check cache → If miss, query DB → Update cache
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
    // Try cache first
    if user, _ := s.cache.GetUser(ctx, id); user != nil {
        log.Printf("Cache hit for user: %s", id)
        return user, nil
    }
    
    // Cache miss - query DynamoDB
    log.Printf("Cache miss for user: %s", id)
    user, err := s.dynamoRepo.GetUser(ctx, id)
    
    // Store in cache for next time
    s.cache.SetUser(ctx, user)
    return user, nil
}

// Update: Update DB → Update cache → Invalidate list
func (s *UserService) UpdateUser(ctx, id, req) (*models.User, error)

// Delete: Delete from DB → Delete from cache → Invalidate list
func (s *UserService) DeleteUser(ctx context.Context, id string) error

// List: Check cache → If miss, query DB → Cache results
func (s *UserService) ListUsers(ctx context.Context) ([]*models.User, error)