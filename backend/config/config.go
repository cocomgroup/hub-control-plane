package config

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

type Config struct {
	Port               string
	AWSRegion          string
	DynamoDBTableName  string
	ContactTableName   string
	RedisAddress       string
	RedisPassword      string
	CacheTTL           int
}

func LoadConfig() *Config {
	return &Config{
		Port:               getEnv("PORT", "8081"),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		DynamoDBTableName:  getEnv("DYNAMODB_TABLE_NAME", "application-table"),
		RedisAddress:       getEnv("REDIS_ADDRESS", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		CacheTTL:           300, // 5 minutes default
	}
}

func NewAWSConfig(region string) aws.Config {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		log.Fatalf("Unable to load AWS SDK config: %v", err)
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}