package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	// Local packages
	"hub-control-plane/backend/config"
	"hub-control-plane/backend/repository"
	"hub-control-plane/backend/service"
	"hub-control-plane/backend/handlers"
)

func main() {
	// Load configuration from environment variables
	cfg := config.LoadConfig()
	log.Printf("Starting server with config: Port=%s, Region=%s", cfg.Port, cfg.AWSRegion)

	// Initialize AWS SDK configuration
	// This loads credentials from environment, IAM role, or AWS config files
	awsConfig := config.NewAWSConfig(cfg.AWSRegion)
	
	// ==========================================
	// REPOSITORY LAYER - Data Access
	// ==========================================
	
	// Initialize User DynamoDB Repository
	// This creates a concrete implementation of UserRepository interface
	// Pattern: NewXxxRepository(dependencies...) returns *XxxRepository
	userRepo := repository.NewDynamoDBRepository(awsConfig, cfg.DynamoDBTableName)
	log.Printf("✓ User DynamoDB repository initialized (table: %s)", cfg.DynamoDBTableName)
	
	// Initialize Contact DynamoDB Repository
	// Separate repository for contacts with its own table
	contactRepo := repository.NewDynamoDBRepository(awsConfig, cfg.ContactTableName)
	log.Printf("✓ Contact DynamoDB repository initialized (table: %s)", cfg.ContactTableName)
	
	// ==========================================
	// CACHE LAYER - Performance Optimization
	// ==========================================
	
	// Initialize Redis Cache for Users
	// This creates a Redis client and wraps it with user-specific cache methods
	redisCfg := repository.RedisConfig{
		Address:  cfg.RedisAddress,
		Password: cfg.RedisPassword,
	}
	
	userCache := repository.NewRedisCache(redisCfg)
	log.Printf("✓ User Redis cache initialized (address: %s)", cfg.RedisAddress)
	
	// Initialize Redis Cache for Contacts
	// Reuses the same Redis connection but with contact-specific keys
	// Pattern: Share the underlying client for efficiency
	contactCache := repository.NewContactRedisCache(userCache.GetClient())
	log.Printf("✓ Contact Redis cache initialized")
	
	// ==========================================
	// SERVICE LAYER - Business Logic
	// ==========================================
	
	// Initialize User Service
	// Dependency Injection: Pass in both repository and cache
	// The service coordinates between cache and database
	userService := service.NewUserService(userRepo, userCache)
	log.Printf("✓ User service initialized")
	
	// Initialize Contact Service
	// Same pattern: inject repository and cache dependencies
	contactService := service.NewContactService(contactRepo, contactCache)
	log.Printf("✓ Contact service initialized")
	
	// ==========================================
	// HANDLER LAYER - HTTP Controllers
	// ==========================================
	
	// Initialize User Handler
	// Dependency Injection: Pass in the service
	// Handler focuses only on HTTP concerns (request/response)
	userHandler := handlers.NewUserHandler(userService)
	log.Printf("✓ User handler initialized")
	
	// Initialize Contact Handler
	contactHandler := handlers.NewContactHandler(contactService)
	log.Printf("✓ Contact handler initialized")

	// ==========================================
	// HTTP SERVER SETUP
	// ==========================================
	
	// Setup router with all handlers
	router := setupRouter(userHandler, contactHandler)
	log.Printf("✓ Router configured with all endpoints")

	// Create HTTP server with configured handler
	srv := &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in a goroutine (non-blocking)
	go func() {
		log.Printf("🚀 Server starting on port %s", cfg.Port)
		log.Printf("📍 Health check: http://localhost:%s/health", cfg.Port)
		log.Printf("📍 API docs: http://localhost:%s/api/v1", cfg.Port)
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Failed to start server: %v", err)
		}
	}()

	// ==========================================
	// GRACEFUL SHUTDOWN
	// ==========================================
	
	// Wait for interrupt signal to gracefully shutdown the server
	// SIGINT = Ctrl+C, SIGTERM = kill command
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🛑 Shutting down server...")

	// Graceful shutdown with 5 second timeout
	// This allows existing requests to complete
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("❌ Server forced to shutdown:", err)
	}

	log.Println("✅ Server exited gracefully")
}

// setupRouter configures all HTTP routes and middleware
func setupRouter(userHandler *handlers.UserHandler, contactHandler *handlers.ContactHandler) *gin.Engine {
	// Create router with default middleware (logger and recovery)
	router := gin.Default()

	// ==========================================
	// HEALTH CHECK ENDPOINT
	// ==========================================
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC(),
			"service":   "go-aws-backend",
			"version":   "1.0.0",
		})
	})

	// ==========================================
	// API v1 ROUTES
	// ==========================================
	v1 := router.Group("/api/v1")
	{
		// ==========================================
		// USER ROUTES
		// ==========================================
		users := v1.Group("/users")
		{
			// Basic CRUD operations
			users.POST("", userHandler.CreateUser)           // Create new user
			users.GET("/:id", userHandler.GetUser)           // Get user by ID
			users.PUT("/:id", userHandler.UpdateUser)        // Update user
			users.DELETE("/:id", userHandler.DeleteUser)     // Delete user
			users.GET("", userHandler.ListUsers)             // List all users
			
			// User's contacts (nested routes)
			users.GET("/:userId/contacts", contactHandler.ListContactsByUser)           // Get all contacts for a user
			users.GET("/:userId/contacts/favorites", contactHandler.ListFavoriteContacts) // Get favorite contacts
		}
		
		// ==========================================
		// CONTACT ROUTES
		// ==========================================
		contacts := v1.Group("/contacts")
		{
			// Basic CRUD operations
			contacts.POST("", contactHandler.CreateContact)      // Create new contact
			contacts.GET("/:id", contactHandler.GetContact)      // Get contact by ID
			contacts.PUT("/:id", contactHandler.UpdateContact)   // Update contact
			contacts.DELETE("/:id", contactHandler.DeleteContact) // Delete contact
			contacts.GET("", contactHandler.ListContacts)        // List all contacts
		}
	}

	return router
}

// ==========================================
// DEPENDENCY INJECTION EXPLANATION
// ==========================================
/*

This application uses Manual Dependency Injection pattern:

FLOW:
  Config → AWS SDK → Repositories → Services → Handlers → Router → Server

BENEFITS:
  1. Testability: Easy to mock dependencies in tests
  2. Flexibility: Can swap implementations (e.g., MySQL instead of DynamoDB)
  3. Clarity: Dependencies are explicit, not hidden
  4. No Magic: No reflection or runtime discovery

EXAMPLE INITIALIZATION CHAIN FOR USER:

  1. awsConfig = config.NewAWSConfig(region)
     └─> Creates AWS SDK configuration
  
  2. userRepo = repository.NewDynamoDBRepository(awsConfig, tableName)
     └─> Creates DynamoDB client
     └─> Implements UserRepository interface
  
  3. userCache = repository.NewRedisCache(address, password)
     └─> Creates Redis client
     └─> Implements UserCache interface
  
  4. userService = service.NewUserService(userRepo, userCache)
     └─> Receives both repository and cache
     └─> Implements business logic
     └─> Coordinates cache-aside pattern
  
  5. userHandler = handlers.NewUserHandler(userService)
     └─> Receives service
     └─> Handles HTTP requests/responses
     └─> Validates input
     └─> Returns JSON

USAGE IN MAIN:

  // Create repositories (data access layer)
  userRepo := repository.NewDynamoDBRepository(awsConfig, cfg.DynamoDBTableName)
  
  // Create cache layer
  userCache := repository.NewRedisCache(cfg.RedisAddress, cfg.RedisPassword)
  
  // Create service (business logic) - inject dependencies
  userService := service.NewUserService(userRepo, userCache)
  
  // Create handler (HTTP layer) - inject service
  userHandler := handlers.NewUserHandler(userService)
  
  // Setup routes - inject handler
  router := setupRouter(userHandler, contactHandler)

ALTERNATIVE APPROACHES:

  1. Dependency Injection Container (e.g., dig, wire)
     - More complex, uses code generation
     - Better for large applications
  
  2. Service Locator Pattern
     - Global registry of services
     - Less explicit dependencies
  
  3. Constructor Injection (what we use)
     - Simple and explicit
     - Perfect for Go applications

*/