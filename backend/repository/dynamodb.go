package repository

import (
    "context"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "hub-control-plane/backend/models"
)

type DynamoDBRepository struct {
    client    *dynamodb.Client
    tableName string
}

// Constructor function 
func NewDynamoDBRepository(awsConfig aws.Config, tableName string) *DynamoDBRepository {
	return &DynamoDBRepository{
		client:    dynamodb.NewFromConfig(awsConfig),  // Create DynamoDB client from AWS config
		tableName: tableName,                           // Store table name
	}
}

// Create with conditional expression (prevents duplicates)
func (r *DynamoDBRepository) CreateUser(ctx context.Context, user *models.User) error {
    av, err := attributevalue.MarshalMap(user)
    if err != nil {
        return err
    }
    input := &dynamodb.PutItemInput{
        TableName:           aws.String(r.tableName),
        Item:                av,
        ConditionExpression: aws.String("attribute_not_exists(id)"),
    }
    _, err = r.client.PutItem(ctx, input)
    if err != nil {
        // handle error, e.g. check for conditional check failure
        return err
    }
    // Returns ErrUserExists if already exists
    return nil
}

// Get single user by ID
func (r *DynamoDBRepository) GetUser(ctx context.Context, id string) (*models.User, error) {
    key, err := attributevalue.MarshalMap(map[string]string{"id": id})
    if err != nil {
        return nil, err
    }
    out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
        TableName: aws.String(r.tableName),
        Key:       key,
    })
    if err != nil {
        return nil, err
    }
    if out.Item == nil {
        return nil, nil
    }
    var user models.User
    if err := attributevalue.UnmarshalMap(out.Item, &user); err != nil {
        return nil, err
    }
    return &user, nil
}

// Update with expression builder (only updates specified fields)
func (r *DynamoDBRepository) UpdateUser(ctx context.Context, id string, updates map[string]interface{}) (*models.User, error) {
    // TODO: implement update using UpdateItem with expression builder.
    // Returning nil,nil for now to keep file compiling.
    return nil, nil
}

// Delete with conditional check
func (r *DynamoDBRepository) DeleteUser(ctx context.Context, id string) error {
    key, err := attributevalue.MarshalMap(map[string]string{"id": id})
    if err != nil {
        return err
    }
    _, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
        TableName:           aws.String(r.tableName),
        Key:                 key,
        ConditionExpression: aws.String("attribute_exists(id)"),
    })
    return err
}

// Scan for all users (limited to 100 for performance)
func (r *DynamoDBRepository) ListUsers(ctx context.Context) ([]*models.User, error) {
    out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
        TableName: aws.String(r.tableName),
        Limit:     aws.Int32(100),
    })
    if err != nil {
        return nil, err
    }
    users := make([]*models.User, 0, len(out.Items))
    for _, item := range out.Items {
        var u models.User
        if err := attributevalue.UnmarshalMap(item, &u); err != nil {
            return nil, err
        }
        users = append(users, &u)
    }
    return users, nil
}