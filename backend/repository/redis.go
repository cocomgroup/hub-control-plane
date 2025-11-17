package repository

import (
    "context"
    "time"
    "encoding/json"

    "github.com/redis/go-redis/v9"
    "hub-control-plane/backend/models"
)

type RedisCache struct {
    client *redis.Client
    ttl    time.Duration  // 5 minutes
}

// Config struct for Redis
type RedisConfig struct {
    Address  string
    Password string
    TTL      time.Duration
}

// Constructor function 
func NewRedisCache(cfg RedisConfig) *RedisCache {
    client := redis.NewClient(&redis.Options{
        Addr:     cfg.Address,
        Password: cfg.Password,
        DB:       0,
    })
    return &RedisCache{
        client: client,
        ttl:    cfg.TTL,
    }
}

// Get user from cache (returns nil if not found)
func (c *RedisCache) GetUser(ctx context.Context, id string) (*models.User, error) {
    val, err := c.client.Get(ctx, c.userKey(id)).Result()
    if err == redis.Nil {
        return nil, nil  // Cache miss
    }
    if err != nil {
        return nil, err
    }
    // Unmarshal JSON to User struct
    var user models.User
    if err := json.Unmarshal([]byte(val), &user); err != nil {
        return nil, err
    }
    return &user, nil
}

// Set user in cache with TTL
func (c *RedisCache) SetUser(ctx context.Context, user *models.User) error {
    // TODO: implement storing the user in redis cache with TTL
    // Minimal stub to keep the code compiling.
    return nil
}

// Delete single user from cache
func (c *RedisCache) DeleteUser(ctx context.Context, id string) error {
    return c.client.Del(ctx, c.userKey(id)).Err()
}

// Invalidate the entire user list cache
func (c *RedisCache) InvalidateUserList(ctx context.Context) error {
    // TODO: implement cache invalidation logic
    return nil
}

// Cache user list
func (c *RedisCache) GetUserList(ctx context.Context) ([]*models.User, error) {
    // TODO: implement retrieval of user list from cache
    return nil, nil
}

func (c *RedisCache) SetUserList(ctx context.Context, users []*models.User) error {
    // TODO: implement storing user list in cache with TTL
    return nil
}

// Generate cache keys: "user:{id}" or "users:list"
func (c *RedisCache) userKey(id string) string {
    return "user:" + id
}