// You should not need to edit this file unless you need to debug something
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/exp/slices"
)

var (
	ctx        = context.Background()
	pool       *pgxpool.Pool
	client     *mongo.Client
	backupList []string
	tagCache   map[string][2][]string = make(map[string][2][]string)
	debug      bool
)

type gfunc struct {
	param    string
	function func(p any) any
}

type backupOpts struct {
	IgnoreFKError     bool
	IgnoreUniqueError bool
	RenameTo          string
	IndexCols         []string
}

type schemaOpts struct {
	TableName string
}

func getTag(field reflect.StructField) (json []string, bson []string) {
	if v, ok := tagCache[field.Name]; ok {
		return v[0], v[1]
	}

	tagSplit := strings.Split(field.Tag.Get("bson"), ",")
	jsonKeyName := strings.Split(field.Tag.Get("json"), ",")

	if len(tagSplit) == 0 {
		panic("No tag found for " + field.Name)
	}

	if len(jsonKeyName) < 1 {
		panic("No json key name found for bson tag at field " + field.Name)
	}

	if jsonKeyName[0] == "-" {
		jsonKeyName[0] = tagSplit[0]
	}

	if jsonKeyName[0] == "" || jsonKeyName[0] == "-" {
		panic("No json key name found for bson tag at field " + field.Name)
	}

	var cond string

	if len(tagSplit) == 1 || field.Tag.Get("notnull") == "true" {
		// No omitempty, so we assume not null
		cond = "not null"
	}

	fieldType := field.Type.Name()

	if fieldType == "" {
		// its a pointer, resolve it
		fieldType = field.Type.Elem().Name()
	}

	// Handle string -> text
	if fieldType == "string" {
		fieldType = "text"
	}

	// Handle bool as boolean
	if fieldType == "bool" {
		fieldType = "boolean"
	}

	// Handle the other int types
	if fieldType == "int" {
		fieldType = "integer"
	} else if fieldType == "int8" {
		fieldType = "smallint"
	} else if fieldType == "int16" {
		fieldType = "smallint"
	} else if fieldType == "int32" {
		fieldType = "integer"
	} else if fieldType == "int64" {
		fieldType = "bigint"
	}

	// Time is timestamptz
	if fieldType == "Time" {
		fieldType = "timestamptz"
	}

	if field.Type.Kind() == reflect.Slice || field.Tag.Get("tolist") == "true" {
		fieldType += "[]"
	}

	if field.Type.Kind() == reflect.Map {
		fieldType = "jsonb" // All maps are assumed to be jsonb
	}

	if field.Tag.Get("mark") != "" {
		fieldType = field.Tag.Get("mark")
	}

	tagCache[field.Name] = [2][]string{{jsonKeyName[0], fieldType + " " + cond}, {tagSplit[0], fieldType + " " + cond}}

	return []string{jsonKeyName[0], fieldType + " " + cond}, []string{tagSplit[0], fieldType + " " + cond}
}

func resolveInput(input string) any {
	if input == "null" {
		return nil
	} else if input == "true" {
		return true
	} else if input == "false" {
		return false
	}

	return input
}

func backupTool(schemaName string, schema any, opts backupOpts) {
	tagCache = make(map[string][2][]string)

	if len(backupList) != 0 && !slices.Contains(backupList, schemaName) {
		fmt.Println("Skipping backup of", schemaName)
		return
	}

	db := client.Database("infinity")

	cur, err := db.Collection(schemaName).Find(ctx, bson.M{})

	count, cerr := db.Collection(schemaName).CountDocuments(ctx, bson.M{})

	if err != nil {
		panic(err)
	}

	if cerr != nil {
		panic(cerr)
	}

	notifyMsg("info", "Backing up "+schemaName)

	if len(backupList) != 0 {
		// Try deleting but ignore if delete fails
		_, err = pool.Exec(ctx, "DROP TABLE "+schemaName)

		if err != nil {
			notifyMsg("error", "Failed to drop table "+schemaName+": "+err.Error())
		}
	}

	_, pgerr := pool.Exec(ctx, "CREATE TABLE "+schemaName+" (itag UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4())")

	if err != nil {
		panic(err)
	}

	if pgerr != nil {
		panic(pgerr)
	}

	structType := reflect.TypeOf(schema)

	for _, field := range reflect.VisibleFields(structType) {
		tag, _ := getTag(field) // We want json tag here as it has what we need
		notifyMsg("debug", fmt.Sprintln("Got tag of", tag, "for field ", field.Name))

		var (
			defaultVal = ""
			uniqueVal  = ""
		)

		if field.Tag.Get("unique") == "true" {
			notifyMsg("debug", fmt.Sprintln("Field", field.Name, "is unique"))
			uniqueVal = " UNIQUE "
		}

		if field.Tag.Get("default") != "" {
			defaultVal = field.Tag.Get("default")

			if strings.HasPrefix(defaultVal, "{}") {
				defaultVal = "'" + defaultVal + "'"
			}

			if strings.Contains(defaultVal, "uuid_generate_v4()") {
				defaultVal = "uuid_generate_v4()"
			}

			if defaultVal == "SKIP" {
				defaultVal = ""
			} else {
				defaultVal = " DEFAULT " + defaultVal
			}
		}

		// Create column
		_, err := pool.Exec(ctx, "ALTER TABLE "+schemaName+" ADD COLUMN "+tag[0]+" "+strings.Join(tag[1:], " ")+uniqueVal+defaultVal)
		if err != nil {
			panic(err)
		}

		// Check for fkey, if so add it
		if field.Tag.Get("fkey") != "" {
			// Format for fkey is REFER_TABLE_NAME,COLUMN_NAME
			fkeySplit := strings.Split(field.Tag.Get("fkey"), ",")
			fkeyRefersParentTable := fkeySplit[0]
			fkeyRefersParentColumn := fkeySplit[1]

			_, err := pool.Exec(ctx, "ALTER TABLE "+schemaName+" ADD CONSTRAINT "+tag[0]+"_fkey FOREIGN KEY ("+tag[0]+") REFERENCES "+fkeyRefersParentTable+"("+fkeyRefersParentColumn+") ON DELETE CASCADE ON UPDATE CASCADE")

			if err != nil {
				panic(err)
			}
		}
	}

	if len(opts.IndexCols) > 0 {
		// Create index on these columns
		colList := strings.Join(opts.IndexCols, ",")
		indexName := schemaName + "_migindex"
		sqlStr := "CREATE INDEX " + indexName + " ON " + schemaName + "(" + colList + ")"

		_, pgerr = pool.Exec(ctx, sqlStr)

		if pgerr != nil {
			panic(pgerr)
		}
	}

	var counter int

	sendProgress(0, int(count), schemaName)

	for cur.Next(ctx) {
		counter++

		sendProgress(counter, int(count), schemaName)

		var result bson.M

		err := cur.Decode(&result)
		if err != nil {
			panic(err)
		}

		var sqlStr string = "INSERT INTO " + schemaName + " ("

		for _, field := range reflect.VisibleFields(structType) {
			if field.Tag.Get("omit") == "true" {
				continue
			}
			tag, _ := getTag(field) // Json tag here again

			sqlStr += tag[0] + ","
		}

		sqlStr = sqlStr[:len(sqlStr)-1] + ") VALUES ("

		args := make([]any, 0)

		argNums := []string{}

		var i int

		var skipped bool

		for _, field := range reflect.VisibleFields(structType) {
			if field.Tag.Get("omit") == "true" {
				continue
			}

			tag, btag := getTag(field) // Here we need both
			if debug {
				notifyMsg("debug", "Table:"+schemaName+"\nField:"+field.Name+"\nType:"+tag[1]+"\n")
			}

			var res any

			res = result[btag[0]]

			if res == "" {
				res = nil
			}

			if res == nil {
				if field.Tag.Get("defaultfunc") != "" {
					fn := exportedFuncs[field.Tag.Get("defaultfunc")]
					if fn == nil {
						panic("Default function " + field.Tag.Get("defaultfunc") + " not found")
					}
					res = fn.function(result[fn.param])
				}
			}

			if field.Tag.Get("pre") != "" {
				fn := exportedFuncs[field.Tag.Get("pre")]
				if fn == nil {
					panic("Pre function " + field.Tag.Get("pre") + " not found")
				}
				res = fn.function(result[fn.param])
			}

			// We have to do this a second time after defaultfunc is called just in case it changed the value back to nil
			if res == nil {
				if field.Tag.Get("default") != "" {
					if strings.Contains(field.Tag.Get("default"), "SKIP") {
						notifyMsg("warning", "Skipping row due to default value at iteration "+strconv.Itoa(counter))
						skipped = true
						continue
					}

					res = resolveInput(field.Tag.Get("default"))
					if resStr, ok := res.(string); ok {
						resStr = strings.TrimPrefix(resStr, "'")
						resStr = strings.TrimSuffix(resStr, "'")

						res = resStr
					}
				} else {
					// Ask user what to do
					var flag bool = true
					for flag {
						fmt.Println("Field", btag[0], "(", tag[0], ") is nil, what do you want to set this to? SKIP to skip this field, or enter a value:")
						var input string
						fmt.Scanln(&input)

						if input == "SKIP" {
							flag = false
							skipped = true
							continue
						}

						res = resolveInput(input)

						if input != "" {
							fmt.Println("Setting", btag[0], "(", tag[0], ") to", input, ". Confirm? (y/n)")
							var confirm string
							fmt.Scanln(&confirm)
							if confirm == "y" {
								flag = false
							}
						}
					}
				}
			}

			if skipped {
				break
			}

			if field.Tag.Get("log") == "1" {
				fmt.Println("Setting", btag[0], "(", tag[0], ") to", res)
			}

			// Handle mark of timestamptz
			if strings.HasPrefix(tag[1], "time") {
				// check if res is int64
				if debug {
					notifyMsg("debug", "Converting a "+reflect.TypeOf(res).Name()+" to time.Time")
				}

				if resCast, ok := res.(int64); ok {
					res = time.UnixMilli(resCast)
				} else if resCast, ok := res.(float64); ok {
					res = time.UnixMilli(int64(resCast))
				} else if resCast, ok := res.(string); ok {
					// Cast string to int64
					resD, err := strconv.ParseInt(resCast, 10, 64)
					if err != nil {
						// Could be a datetime string
						resDV, err := time.Parse(time.RFC3339, resCast)
						if err != nil {
							// Last ditch effort, try checking if its NOW or something
							if strings.Contains(resCast, "NOW") {
								res = time.Now()
							} else {
								panic(err)
							}
						} else {
							res = resDV
						}
					} else {
						res = time.UnixMilli(resD)
					}
				} else if resCast, ok := res.(primitive.DateTime); ok {
					res = time.UnixMilli(resCast.Time().UnixMilli())
				} else if resCast, ok := res.(primitive.A); ok {
					// For each int64 in the array, convert to time.Time
					resV := make([]time.Time, len(resCast))
					for i, v := range resCast {
						if val, ok := v.(int64); ok {
							resV[i] = time.UnixMilli(val)
						} else if val, ok := v.(float64); ok {
							resV[i] = time.UnixMilli(int64(val))
						}
					}
					res = resV
				}
			}

			// Handle tolist
			if field.Tag.Get("tolist") == "true" {
				if resCast, ok := res.(string); ok {
					res = strings.Split(strings.ReplaceAll(resCast, " ", ""), ",")

					if debug {
						notifyMsg("debug", "Converting "+resCast+" to list")
					}
				}
			}

			args = append(args, res)

			argNums = append(argNums, "$"+strconv.Itoa(i+1))

			i++
		}

		if skipped {
			continue
		}

		sqlStr += strings.Join(argNums, ",") + ")"

		if debug {
			notifyMsg("debug", "SQL String: "+sqlStr)
		}

		_, pgerr = pool.Exec(ctx, sqlStr, args...)

		if pgerr != nil {
			if opts.IgnoreFKError && strings.Contains(pgerr.Error(), "violates foreign key") {
				notifyMsg("warning", "Ignoring foreign key error on iter "+strconv.Itoa(counter)+": "+pgerr.Error())
				continue
			} else if opts.IgnoreUniqueError && strings.Contains(pgerr.Error(), "unique constraint") {
				notifyMsg("warning", "Ignoring unique error on iter "+strconv.Itoa(counter)+": "+pgerr.Error())
				continue
			}
			panic(pgerr)
		}
	}

	if opts.RenameTo != "" {
		// Rename postgres table
		sqlStr := "ALTER TABLE " + schemaName + " RENAME TO " + opts.RenameTo
		_, pgerr = pool.Exec(ctx, sqlStr)

		if pgerr != nil {
			panic(pgerr)
		}
	}

	notifyDone(counter, int(count), schemaName)
}

func main() {
	backupCols := flag.String("backup", "", "Which collections to backup. Default is all")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")

	flag.Parse()

	// Check if daemon is running regardless of whether we are backing up a specific schema
	req, err := http.NewRequest("GET", "http://localhost:3939", nil)

	if err != nil {
		panic(err)
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		panic(err)
	}

	if res.StatusCode != 200 {
		panic("Daemon is not running")
	}

	if *backupCols == "" {
		backupList = []string{}
	} else {
		backupList = strings.Split(*backupCols, ",")
	}

	if len(backupList) == 0 {
		fmt.Println("No collections specified, backing up all")
	}

	sendRoutine()

	// Create mongodb conn
	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017/infinity"))

	if err != nil {
		panic(err)
	}

	schemaOpts := getOpts()

	// Create postgres conn
	pool, err = pgxpool.Connect(ctx, "postgresql://127.0.0.1:5432/"+schemaOpts.TableName+"?user=root&password=iblpublic")

	if err != nil {
		panic(err)
	}

	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	if err != nil {
		panic(err)
	}

	if len(backupList) == 0 {
		pool.Exec(ctx, `DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO public;
COMMENT ON SCHEMA public IS 'standard public schema'`)
		pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
	}

	backupSchemas()
}
