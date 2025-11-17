package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)


// DynamoContactRepository implements basic CRUD for Contact in DynamoDB.
type DynamoContactRepository struct {
	db        *dynamodb.Client
	tableName string
	// OwnerIndex is the GSI name that indexes owner_id -> id (optional).
	OwnerIndex string
}

// NewDynamoContactRepository creates a new repository.
// tableName is the DynamoDB table; ownerIndex is the optional GSI name for owner queries (pass "" if none).
func NewDynamoContactRepository(db *dynamodb.Client, tableName string, ownerIndex string) *DynamoContactRepository {
	return &DynamoContactRepository{
		db:         db,
		tableName:  tableName,
		OwnerIndex: ownerIndex,
	}
}

// Create inserts a new contact. It will fail if an item with the same ID exists.
// Create inserts a new contact. It will fail if an item with the same ID exists.
func (r *DynamoContactRepository) Create(ctx context.Context, c *Contact) error {
	// Do not mutate timestamps here; caller should set CreatedAt/UpdatedAt if needed.

	item, err := attributevalue.MarshalMap(c)
	if err != nil {
		return err
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	return err
}
// GetByID fetches a contact by its ID. Returns (nil, nil) if not found.
func (r *DynamoContactRepository) GetByID(ctx context.Context, id string) (*Contact, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	var c Contact
	if err := attributevalue.UnmarshalMap(out.Item, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Update replaces a contact (upserts). UpdatedAt will be set to now.
func (r *DynamoContactRepository) Update(ctx context.Context, c *Contact) error {
	item, err := attributevalue.MarshalMap(c)
	if err != nil {
		return err
	}
	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	return err
}

// ListByOwner queries contacts by owner ID. Requires a GSI on owner_id (set OwnerIndex when creating the repo).
func (r *DynamoContactRepository) ListByOwner(ctx context.Context, ownerID string) ([]*Contact, error) {
	// If no owner index configured, return empty slice.
	if r.OwnerIndex == "" {
		return nil, nil
	}

	av, err := attributevalue.Marshal(ownerID)
	if err != nil {
		return nil, err
	}

	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(r.tableName),
		IndexName:                 aws.String(r.OwnerIndex),
		KeyConditionExpression:    aws.String("owner_id = :owner"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":owner": av},
	})
	if err != nil {
		return nil, err
	}

	contacts := make([]*Contact, 0, len(out.Items))
	for _, it := range out.Items {
		var c Contact
		if err := attributevalue.UnmarshalMap(it, &c); err != nil {
			return nil, err
		}
		contacts = append(contacts, &c)
	}
	return contacts, nil
}