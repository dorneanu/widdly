package dynamodb

import (
	"fmt"
	"log"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"gitlab.com/opennota/widdly/store"
)

type (
	TiddlerRevision struct {
		Key      string
		Revision int
		Text     string
	}
	TiddlerHistory struct {
		tableName string
		store     *dynamodbStore
	}
)

// NewTiddlerHistory returns a pointer to a TiddlerHistory object
func NewTiddlerHistory(store *dynamodbStore, tableName string) *TiddlerHistory {
	return &TiddlerHistory{
		tableName: tableName,
		store:     store,
	}
}

// CreateTable creates the table in which the tiddlers history should be
// stored in. If the table exists, then the method just returns
// with no error
func (t *TiddlerHistory) CreateTable() error {
	// Check if table already exists
	if err := t.store.TableExists(t.tableName); err == true {
		return nil
	}

	log.Printf("Creating table: %s ...", t.tableName)
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(t.store.tableKey),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String(t.store.tableRevisionKey),
				AttributeType: aws.String("N"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(t.store.tableKey),
				KeyType:       aws.String("HASH"),
			},
			{
				AttributeName: aws.String(t.store.tableRevisionKey),
				KeyType:       aws.String("RANGE"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
		TableName: aws.String(t.tableName),
	}

	result, _ := t.store.svc.CreateTable(input)
	log.Printf("Created table: %s\n\n", result)
	return nil
}

// Put creates a new tiddler revision and puts it into the table
func (t *TiddlerHistory) Put(tiddler store.Tiddler, rev int) (int, error) {
	// Create new tiddler with revision
	tiddlerRev := &TiddlerRevision{
		Key:      tiddler.Key,
		Text:     tiddler.Text,
		Revision: rev,
	}
	// Convert tiddler to DynamoDB attributes
	item, err := dynamodbattribute.MarshalMap(tiddlerRev)
	if err != nil {
		return 0, err
	}

	// Put item to history table
	_, err = t.store.svc.PutItem(&dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(t.tableName),
	})
	if err != nil {
		return 0, fmt.Errorf("Couldn't put item, %v", err)
	}

	return rev, nil
}

// Delete deletes a tiddler revision
func (t *TiddlerHistory) Delete(key string, rev int) error {
	_, err := t.store.svc.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			t.store.tableKey: {
				S: aws.String(key),
			},
			t.store.tableRevisionKey: {
				N: aws.String(strconv.Itoa(rev)),
			},
		},
		TableName: aws.String(t.store.tableHistory),
	})
	return err
}
