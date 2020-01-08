package dynamodb

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"gitlab.com/opennota/widdly/store"
)

// TiddlerData is the structure representing the tiddler itself
type TiddlerData struct {
	tableName string
	store     *dynamodbStore
}

// NewTiddlerData returns a pointer to a TiddlerData
func NewTiddlerData(store *dynamodbStore, tableName string) *TiddlerData {
	return &TiddlerData{
		tableName: tableName,
		store:     store,
	}
}

// CreateTable creates the table in which the tiddlers should be
// stored in. If the table exists, then the method just returns
// with no error
func (t *TiddlerData) CreateTable() error {
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
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(t.store.tableKey),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
		TableName: aws.String(t.tableName),
	}
	result, err := t.store.svc.CreateTable(input)
	log.Printf("Created table: %s\n\n", result)
	return err
}

// Put creates a new tiddler and puts it into the table
func (t *TiddlerData) Put(tiddler store.Tiddler) (int, error) {
	// Get meta information first
	jsMeta, err := t.store.GetMeta(tiddler)
	if err != nil {
		return 0, fmt.Errorf("Couldn't get meta from tiddler, %v", err)
	}

	// Get next revision
	nextRev := t.store.NextRevision(tiddler.Key)

	jsMeta["revision"] = strconv.Itoa(nextRev)
	metaData, err := json.Marshal(jsMeta)
	if err != nil {
		log.Panic(fmt.Errorf("Couldn't marshalize json, %v", err))
	}
	tiddler.Meta = metaData

	// Convert tiddler to DynamoDB attributes
	item, err := dynamodbattribute.MarshalMap(tiddler)
	if err != nil {
		log.Panic(err)
		return 0, err
	}

	// Put item to table tiddlers
	_, err = t.store.svc.PutItem(&dynamodb.PutItemInput{
		Item:      item,
		TableName: aws.String(t.tableName),
	})
	if err != nil {
		return 0, fmt.Errorf("Couldn't put item, %v", err)
	}

	return nextRev, nil
}

// Delete deletes a tiddler from the table
func (t *TiddlerData) Delete(key string) error {
	log.Println("Deleting: ", key)
	_, err := t.store.svc.DeleteItem(&dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			t.store.tableKey: {
				S: aws.String(key),
			},
		},
		TableName: aws.String(t.store.tableTiddlers),
	})
	return err
}
