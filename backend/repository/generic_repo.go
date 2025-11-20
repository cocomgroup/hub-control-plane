package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Common errors
var (
	ErrNotFound      = errors.New("item not found")
	ErrAlreadyExists = errors.New("item already exists")
)

// BaseModel interface that all models must implement
// This allows the repository to work with any type
type BaseModel interface {
	GetPK() string           // Partition Key (e.g., "USER#123")
	GetSK() string           // Sort Key (e.g., "METADATA" or "CONTACT#456")
	SetPK(pk string)         // Set partition key
	SetSK(sk string)         // Set sort key
	GetEntityType() string   // Entity type (e.g., "USER", "CONTACT", "ORDER")
}

// GenericRepository - Single table design repository for all entities
type GenericRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewGenericRepository creates a new generic repository
func NewGenericRepository(awsConfig aws.Config, tableName string) *GenericRepository {
	return &GenericRepository{
		client:    dynamodb.NewFromConfig(awsConfig),
		tableName: tableName,
	}
}

// Put creates or updates an item in DynamoDB
// T must implement BaseModel interface
func (r *GenericRepository) Put(ctx context.Context, item BaseModel) error {
	// Add timestamps
	if timestamped, ok := item.(interface{ SetTimestamps() }); ok {
		timestamped.SetTimestamps()
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	}

	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// PutIfNotExists creates an item only if it doesn't exist (prevents overwrites)
func (r *GenericRepository) PutIfNotExists(ctx context.Context, item BaseModel) error {
	// Add timestamps
	if timestamped, ok := item.(interface{ SetTimestamps() }); ok {
		timestamped.SetTimestamps()
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	}

	_, err = r.client.PutItem(ctx, input)
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// Get retrieves an item by PK and SK
// The result parameter must be a pointer to the struct you want to unmarshal into
func (r *GenericRepository) Get(ctx context.Context, pk, sk string, result BaseModel) error {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	}

	output, err := r.client.GetItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to get item: %w", err)
	}

	if output.Item == nil {
		return ErrNotFound
	}

	if err := attributevalue.UnmarshalMap(output.Item, result); err != nil {
		return fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return nil
}

// Update updates specific attributes of an item
func (r *GenericRepository) Update(ctx context.Context, pk, sk string, updates map[string]interface{}) error {
	// Add updated_at timestamp
	updates["UpdatedAt"] = time.Now().UTC()

	// Build update expression
	update := expression.UpdateBuilder{}
	for key, value := range updates {
		update = update.Set(expression.Name(key), expression.Value(value))
	}

	expr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       aws.String("attribute_exists(PK)"),
	}

	_, err = r.client.UpdateItem(ctx, input)
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// Delete removes an item from DynamoDB
func (r *GenericRepository) Delete(ctx context.Context, pk, sk string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		ConditionExpression: aws.String("attribute_exists(PK)"),
	}

	_, err := r.client.DeleteItem(ctx, input)
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// Query queries items by PK (and optionally SK prefix)
func (r *GenericRepository) Query(ctx context.Context, pk string, skPrefix string, resultSlice interface{}) error {
	var keyCondition expression.KeyConditionBuilder
	
	if skPrefix == "" {
		// Query all items with this PK
		keyCondition = expression.Key("PK").Equal(expression.Value(pk))
	} else {
		// Query items with PK and SK prefix
		keyCondition = expression.Key("PK").Equal(expression.Value(pk)).
			And(expression.Key("SK").BeginsWith(skPrefix))
	}

	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	output, err := r.client.Query(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to query items: %w", err)
	}

	if err := attributevalue.UnmarshalListOfMaps(output.Items, resultSlice); err != nil {
		return fmt.Errorf("failed to unmarshal items: %w", err)
	}

	return nil
}

// QueryByEntityType queries items by entity type using GSI1
func (r *GenericRepository) QueryByEntityType(ctx context.Context, entityType string, resultSlice interface{}) error {
	keyCondition := expression.Key("GSI1PK").Equal(expression.Value(entityType))

	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		IndexName:                 aws.String("GSI1"),
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	output, err := r.client.Query(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to query by entity type: %w", err)
	}

	if err := attributevalue.UnmarshalListOfMaps(output.Items, resultSlice); err != nil {
		return fmt.Errorf("failed to unmarshal items: %w", err)
	}

	return nil
}

// QueryWithFilter queries with additional filter conditions
func (r *GenericRepository) QueryWithFilter(
	ctx context.Context,
	pk string,
	skPrefix string,
	filterCondition expression.ConditionBuilder,
	resultSlice interface{},
) error {
	var keyCondition expression.KeyConditionBuilder
	
	if skPrefix == "" {
		keyCondition = expression.Key("PK").Equal(expression.Value(pk))
	} else {
		keyCondition = expression.Key("PK").Equal(expression.Value(pk)).
			And(expression.Key("SK").BeginsWith(skPrefix))
	}

	expr, err := expression.NewBuilder().
		WithKeyCondition(keyCondition).
		WithFilter(filterCondition).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build expression: %w", err)
	}

	input := &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	}

	output, err := r.client.Query(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to query with filter: %w", err)
	}

	if err := attributevalue.UnmarshalListOfMaps(output.Items, resultSlice); err != nil {
		return fmt.Errorf("failed to unmarshal items: %w", err)
	}

	return nil
}

// BatchGet retrieves multiple items by their keys
func (r *GenericRepository) BatchGet(ctx context.Context, keys []map[string]string, resultSlice interface{}) error {
	if len(keys) == 0 {
		return nil
	}

	// Convert keys to DynamoDB format
	dynamoKeys := make([]map[string]types.AttributeValue, len(keys))
	for i, key := range keys {
		dynamoKeys[i] = map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: key["PK"]},
			"SK": &types.AttributeValueMemberS{Value: key["SK"]},
		}
	}

	input := &dynamodb.BatchGetItemInput{
		RequestItems: map[string]types.KeysAndAttributes{
			r.tableName: {
				Keys: dynamoKeys,
			},
		},
	}

	output, err := r.client.BatchGetItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to batch get items: %w", err)
	}

	items := output.Responses[r.tableName]
	if err := attributevalue.UnmarshalListOfMaps(items, resultSlice); err != nil {
		return fmt.Errorf("failed to unmarshal items: %w", err)
	}

	return nil
}

// BatchWrite performs batch write operations (Put/Delete)
func (r *GenericRepository) BatchWrite(ctx context.Context, putItems []BaseModel, deleteKeys []map[string]string) error {
	writeRequests := make([]types.WriteRequest, 0)

	// Add put requests
	for _, item := range putItems {
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}

		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: av,
			},
		})
	}

	// Add delete requests
	for _, key := range deleteKeys {
		writeRequests = append(writeRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: key["PK"]},
					"SK": &types.AttributeValueMemberS{Value: key["SK"]},
				},
			},
		})
	}

	// DynamoDB batch write limit is 25 items
	for i := 0; i < len(writeRequests); i += 25 {
		end := i + 25
		if end > len(writeRequests) {
			end = len(writeRequests)
		}

		batch := writeRequests[i:end]
		input := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				r.tableName: batch,
			},
		}

		_, err := r.client.BatchWriteItem(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to batch write items: %w", err)
		}
	}

	return nil
}

// Transaction performs a transactional write
func (r *GenericRepository) Transaction(ctx context.Context, puts []BaseModel, deletes []map[string]string) error {
	transactItems := make([]types.TransactWriteItem, 0)

	// Add put transactions
	for _, item := range puts {
		av, err := attributevalue.MarshalMap(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}

		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(r.tableName),
				Item:      av,
			},
		})
	}

	// Add delete transactions
	for _, key := range deletes {
		transactItems = append(transactItems, types.TransactWriteItem{
			Delete: &types.Delete{
				TableName: aws.String(r.tableName),
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: key["PK"]},
					"SK": &types.AttributeValueMemberS{Value: key["SK"]},
				},
			},
		})
	}

	input := &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	}

	_, err := r.client.TransactWriteItems(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	return nil
}