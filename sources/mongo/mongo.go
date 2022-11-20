// Implements both Source and BackupSource
package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/exp/slices"
)

var ctx = context.Background()

type MongoSource struct {
	ConnectionURL  string
	DatabaseName   string
	Conn           *mongo.Client
	Database       *mongo.Database
	connected      bool
	IgnoreEntities []string
}

func (m *MongoSource) Connect() error {
	var err error
	m.Conn, err = mongo.Connect(ctx, options.Client().ApplyURI(m.ConnectionURL))
	if err != nil {
		return err
	}
	m.Database = m.Conn.Database(m.DatabaseName)
	m.connected = true
	return nil
}

func (m MongoSource) RecordList() ([]string, error) {
	if !m.connected {
		return nil, errors.New("not connected")
	}

	var record []string
	cur, err := m.Database.ListCollectionNames(ctx, bson.M{})

	if err != nil {
		return nil, err
	}

	for _, v := range cur {
		if !slices.Contains(m.IgnoreEntities, v) {
			record = append(record, v)
		}
	}

	return record, nil
}

func (m MongoSource) GetRecords(entity string) ([]map[string]any, error) {
	if slices.Contains(m.IgnoreEntities, entity) {
		return []map[string]any{}, nil
	}

	if !m.connected {
		return nil, errors.New("not connected")
	}

	var record []map[string]any
	cur, err := m.Database.Collection(entity).Find(ctx, bson.M{})

	if err != nil {
		return nil, err
	}

	defer cur.Close(ctx)

	for cur.Next(ctx) {
		var mongoEntity bson.M
		err = cur.Decode(&mongoEntity)
		if err != nil {
			return nil, err
		}
		record = append(record, mongoEntity)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return record, nil
}

func (m MongoSource) GetCount(entity string) (int64, error) {
	if slices.Contains(m.IgnoreEntities, entity) {
		return 0, nil
	}

	intVal, err := m.Database.Collection(entity).CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, err
	}
	return intVal, nil
}

// Special mongo specific types
func (m MongoSource) ExtParse(res any) (any, error) {
	var result any
	if resCast, ok := res.(primitive.DateTime); ok {
		result = time.UnixMilli(resCast.Time().UnixMilli())
	} else if resCast, ok := res.(primitive.A); ok {
		if len(resCast) > 0 {
			// We can try doing some smart type inference here
			switch resCast[0].(type) {
			case primitive.DateTime:
				var resultV = []time.Time{}

				for _, v := range resCast {
					resultV = append(resultV, v.(primitive.DateTime).Time())
				}

				result = resultV
				return result, nil
			case int64:
				var resultV = []int64{}

				for _, v := range resCast {
					resultV = append(resultV, v.(int64))
				}

				result = resultV
				return result, nil
			case float64:
				var resultV = []float64{}

				for _, v := range resCast {
					resultV = append(resultV, v.(float64))
				}

				result = resultV
				return result, nil
			}
		}

		// Fallback to string
		if resCast == nil {
			result = []string{}
		}

		var resultArr = []string{}
		for _, v := range resCast {
			resultArr = append(resultArr, fmt.Sprint(v))
		}

		result = resultArr
	} else {
		return result, errors.New("no external representation for type")
	}
	return result, nil
}
