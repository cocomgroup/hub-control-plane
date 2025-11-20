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
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"

	// Local packages
	"hub-control-plane/backend/config"
	"hub-control-plane/backend/repository"
	"hub-control-plane/backend/graphql"
	"hub-control-plane/backend/graphql/resolvers"
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
	repo := repository.NewGenericRepository(awsConfig, cfg.DynamoDBTableName)
	log.Printf("✓ DynamoDB generic repository initialized (table: %s)", cfg.DynamoDBTableName)
	
	// ==========================================
	// CACHE LAYER - Performance Optimization
	// ==========================================
	
	// Initialize Redis Cache for Users
	// This creates a Redis client and wraps it with user-specific cache methods
	cache := repository.NewRedisCache(cfg.RedisAddress, cfg.RedisPassword)
	log.Printf("✓ User Redis cache initialized (address: %s)", cfg.RedisAddress)
	redisClient := cache.GetClient() 
	
	// ==========================================
	// SERVICE LAYER - Business Logic
	// ==========================================
	
	// Initialize User Service
	// Dependency Injection: Pass in both repository and cache
	// The service coordinates between cache and database
	appService := service.NewAppServiceWithCache(repo, redisClient)
	log.Printf("✓ App service initialized")
	
	// Create app handler for REST API
	appHandler := handlers.NewAppHandler(appService)
	log.Printf("✓ App handler initialized")

	// ==========================================
	// GRAPHQL SETUP
	// ==========================================
	
	// Create GraphQL resolver
	gqlResolver := resolvers.NewResolver(appService)
	log.Printf("✓ GraphQL resolver initialized")
	
	// Create GraphQL server
	gqlServer := handler.NewDefaultServer(
		graphql.NewExecutableSchema(
			graphql.Config{Resolvers: gqlResolver},
		),
	)
	log.Printf("✓ GraphQL server initialized")

	// ==========================================
	// HTTP SERVER SETUP
	// ==========================================
	
	// Setup router with all handlers
	router := setupRouter(appHandler, gqlServer)
	log.Printf("✓ Router configured")

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
func setupRouter(
    appHandler *handlers.AppHandler,
    gqlServer *handler.Server,
) *gin.Engine {
    router := gin.Default()

    // ==========================================
    // HEALTH CHECK ENDPOINT
    // ==========================================
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status":    "healthy",
            "timestamp": time.Now().UTC(),
            "service":   "hub-control-plane",
            "version":   "2.0.0",
            "apis":      []string{"REST", "GraphQL"},
        })
    })

    // ==========================================
    // GRAPHQL ENDPOINTS
    // ==========================================
    
    // GraphQL API endpoint
    router.POST("/graphql", gin.WrapH(gqlServer))
    router.GET("/graphql", gin.WrapH(gqlServer))
    
    // GraphQL Playground (development tool)
    router.GET("/playground", gin.WrapH(playground.Handler("GraphQL Playground", "/graphql")))

    // ==========================================
    // REST API ENDPOINTS (v1)
    // ==========================================
    v1 := router.Group("/api/v1")
    {
        // User routes
        users := v1.Group("/users")
        {
            users.POST("", appHandler.CreateUser)
			users.GET("", appHandler.ListUsers)
            users.GET("/:id", appHandler.GetUser)
            users.PUT("/:id", appHandler.UpdateUser)
            users.DELETE("/:id", appHandler.DeleteUser)
        }
        
        // Contact routes - using :id for userId to keep RESTful
        userContacts := v1.Group("/users/:id")
        {
			userContacts.POST("/contacts", appHandler.CreateContact)
			userContacts.GET("/contacts", appHandler.ListUserContacts)
			userContacts.GET("/contacts/favorites", appHandler.ListFavoriteContacts)
			userContacts.GET("/contacts/:contactId", appHandler.GetContact)
			userContacts.PUT("/contacts/:contactId", appHandler.UpdateContact)
			userContacts.DELETE("/contacts/:contactId", appHandler.DeleteContact)
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