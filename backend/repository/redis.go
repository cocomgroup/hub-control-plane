package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

    "github.com/redis/go-redis/v9"
    "hub-control-plane/backend/models"
)

type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisCache(address, password string) *RedisCache {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: password,
		DB:       0, // use default DB
	})

	return &RedisCache{
		client: client,
		ttl:    5 * time.Minute, // default TTL
	}
}

// Ping checks if Redis is connected
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// GetUser retrieves a user from cache
func (c *RedisCache) GetUser(ctx context.Context, id string) (*models.User, error) {
	key := c.userKey(id)
	
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var user models.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// SetUser stores a user in cache
func (c *RedisCache) SetUser(ctx context.Context, user *models.User) error {
	key := c.userKey(user.ID)
	
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	return nil
}

// DeleteUser removes a user from cache
func (c *RedisCache) DeleteUser(ctx context.Context, id string) error {
	key := c.userKey(id)
	
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	return nil
}

// InvalidateUserList invalidates the cached user list
func (c *RedisCache) InvalidateUserList(ctx context.Context) error {
	key := "users:list"
	
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to invalidate list cache: %w", err)
	}

	return nil
}

// GetUserList retrieves the cached user list
func (c *RedisCache) GetUserList(ctx context.Context) ([]*models.User, error) {
	key := "users:list"
	
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get list from cache: %w", err)
	}

	var users []*models.User
	if err := json.Unmarshal([]byte(val), &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}

	return users, nil
}

// SetUserList stores the user list in cache
func (c *RedisCache) SetUserList(ctx context.Context, users []*models.User) error {
	key := "users:list"
	
	data, err := json.Marshal(users)
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set list cache: %w", err)
	}

	return nil
}

// userKey generates the cache key for a user
func (c *RedisCache) userKey(id string) string {
	return fmt.Sprintf("user:%s", id)
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// GetClient returns the underlying Redis client for sharing
func (c *RedisCache) GetClient() *redis.Client {
	return c.client
}