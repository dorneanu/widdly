package dynamodb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"

	"gitlab.com/opennota/widdly/store"
)

// dynamodbStore is a store for tiddlers using AWS DynamoDB
type dynamodbStore struct {
	sess             *session.Session
	svc              dynamodbiface.DynamoDBAPI
	tiddlerData      *TiddlerData
	tiddlerHistory   *TiddlerHistory
	table            string
	tableTiddlers    string
	tableHistory     string
	tableKey         string
	tableRevisionKey string
}

func init() {
	if store.MustOpen != nil {
		panic("attempt to use two different backends at the same time!")
	}
	store.MustOpen = MustOpen
}

// NewDynamodbStore requires an URL to the dynamoDB instance
// and returns an object which implements TiddlerStore
func NewDynamodbStore(url string) *dynamodbStore {
	config := &aws.Config{
		Endpoint: aws.String(url),
	}
	sess := session.Must(session.NewSession(config))

	// Setup dynamoDB client
	svc := dynamodb.New(sess)

	// TODO: Put constants to some configuration
	return &dynamodbStore{
		sess:             sess,
		svc:              svc,
		table:            url,
		tableTiddlers:    "tiddlers",
		tableHistory:     "tiddlers_history",
		tableKey:         "Key",
		tableRevisionKey: "Revision",
	}
}

// MustOpen opens a dynamoDB store at storePath, creating tables if needed,
// and  returns a TiddlerStore.
func MustOpen(dataSource string) store.TiddlerStore {
	store := NewDynamodbStore(dataSource)

	// Create new tiddler data object
	store.tiddlerData = NewTiddlerData(store, store.tableTiddlers)

	// Create new tiddler history
	store.tiddlerHistory = NewTiddlerHistory(store, store.tableHistory)

	// Create tables
	store.CreateTables()
	return store
}

// CreateTables creates the tiddlers and history tables if they don't exist
func (d *dynamodbStore) CreateTables() {
	// Create table tiddlers
	err := d.tiddlerData.CreateTable()
	if err != nil {
		log.Panic(fmt.Errorf("Failed creating tiddlers table, %v", err))
	}

	// Create table tiddler history
	err = d.tiddlerHistory.CreateTable()
	if err != nil {
		log.Panic(fmt.Errorf("Failed creating history table, %v", err))
	}
}

// TableExists will check if a specific table exists in DynamoDB
func (d *dynamodbStore) TableExists(tableName string) bool {
	_, err := d.svc.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		// Casting to the awserr.Error type will allow you to inspect the error
		// code returned by the service in code.
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeResourceNotFoundException:
				log.Print(dynamodb.ErrCodeResourceNotFoundException, aerr.Error())
			case dynamodb.ErrCodeInternalServerError:
				log.Print(dynamodb.ErrCodeInternalServerError, aerr.Error())
			default:
				log.Print(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Print(err.Error())
		}
		return false
	}

	return true
}

// Get retrieves a tiddler from DynamoDB using title as a key
func (d *dynamodbStore) Get(_ context.Context, key string) (store.Tiddler, error) {
	// Try to get tiddler
	result, err := d.svc.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			d.tableKey: {
				S: aws.String(key),
			},
		},
		TableName: aws.String(d.tableTiddlers),
	})
	if err != nil {
		log.Printf("Failed to get tiddler: %v", err)
		return store.Tiddler{}, store.ErrNotFound
	}

	// Create new tiddler
	t := store.Tiddler{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &t)
	if err != nil {
		return store.Tiddler{}, err
	}
	t.WithText = true
	return t, nil
}

// All retrieves all tiddlers from the store
// Special tiddlers (e.g. global macros) are returned fat
func (d *dynamodbStore) All(_ context.Context) ([]store.Tiddler, error) {
	tiddlers := []store.Tiddler{}

	// Make the DynamoDB Query API call
	result, err := d.svc.Scan(&dynamodb.ScanInput{
		TableName: aws.String(d.tableTiddlers),
	})
	if err != nil {
		return tiddlers, fmt.Errorf("Failed to make Query API call, %v", err)
	}

	// Unmarshal the Items field in the result value to the Item Go type.
	err = dynamodbattribute.UnmarshalListOfMaps(result.Items, &tiddlers)
	if err != nil {
		return tiddlers, fmt.Errorf("Failed to unmarshal Query result items, %v", err)
	}

	// Handle special tiddlers
	for _, t := range tiddlers {
		if bytes.Contains(t.Meta, []byte(`"$:/tags/Macro"`)) {
			t.WithText = true
		}
	}

	return tiddlers, nil
}

// Put saves tiddler to the store, incrementing and returning revision.
// The tiddler is also written to the tiddlers_history table.
func (d *dynamodbStore) Put(_ context.Context, tiddler store.Tiddler) (int, error) {
	// Put to tiddlers table
	rev, err := d.tiddlerData.Put(tiddler)
	if err != nil {
		return 0, fmt.Errorf("Couldn't put item into tiddlers table, %v", err)
	}

	// Put to history table
	if !d.skipHistory(tiddler.Key) {
		_, err = d.tiddlerHistory.Put(tiddler, rev)
		if err != nil {
			return 0, fmt.Errorf("Couldn't put item into history table, %v", err)
		}
	}

	return rev, nil
}

// Delete deletes an entry in the DynamoDB determined by key (title of tiddler)
func (d *dynamodbStore) Delete(c context.Context, key string) error {
	// Get tiddler first
	tiddler, err := d.Get(c, key)
	if err != nil {
		return fmt.Errorf("Couldn't get tiddler %s", key)
	}

	// Delete tiddler in data table
	if err := d.tiddlerData.Delete(key); err != nil {
		return fmt.Errorf("Error deleting item %s", key)
	}

	// Get meta information
	meta, err := d.GetMeta(tiddler)
	if err != nil {
		return fmt.Errorf("Couldn't get tiddler meta")
	}

	currentRev, err := strconv.Atoi(meta["revision"].(string))
	if err != nil {
		return fmt.Errorf("Couldn't convert revision to int, %#v", err)
	}

	// Make sure to delete every revision
	for i := 1; i <= currentRev; i++ {
		if err := d.tiddlerHistory.Delete(key, i); err != nil {
			log.Printf("Couldn't delete revision %d for tiddler %s\n", i, key)
		}
	}

	return nil
}

// NextRevision returns next revision for specified tiddler
func (d *dynamodbStore) NextRevision(key string) int {
	defaultRev := 1
	t := store.Tiddler{}

	// Some (special) tiddlers don't need revision
	if d.skipHistory(key) {
		return defaultRev
	}

	// Try to get tiddler from tiddlers table
	result, err := d.svc.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			d.tableKey: {
				S: aws.String(key),
			},
		},
		TableName: aws.String(d.tableTiddlers),
	})

	// Get revision from meta and increment it
	if err != nil {
		log.Printf("Couldn't get tiddler %s: %v", key, err)
		return defaultRev
	}

	if dynamodbattribute.UnmarshalMap(result.Item, &t) == nil {
		jsMeta, err := d.GetMeta(t)
		if err != nil {
			// tiddler doesn't exist yet
			log.Printf("Couldn't get meta from tiddler %s, %v", key, err)
			return defaultRev
		}

		log.Printf("Current Revision %s: %s", key, jsMeta["revision"])
		nextRev, err := strconv.Atoi(jsMeta["revision"].(string))
		if err != nil {
			log.Printf("Couldn't convert revision to int, %#v", err)
			return defaultRev
		}

		// Increase revision
		nextRev++
		return nextRev
	}

	return defaultRev
}

// skipHistory checks if current tiddler (specified by key) is
// the story list or a draft tiddler
func (d *dynamodbStore) skipHistory(key string) bool {
	return key == "$:/StoryList" || strings.HasPrefix(key, "Draft of ")
}

// GetMeta extracts meta information from specified tiddler
func (d *dynamodbStore) GetMeta(tiddler store.Tiddler) (map[string]interface{}, error) {
	var js map[string]interface{}
	if err := json.Unmarshal(tiddler.Meta, &js); err != nil {
		return nil, err
	}
	return js, nil
}
