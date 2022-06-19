// You should not need to edit this file unless you need to debug something
package main

import (
	"context"
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/exp/slices"
)

var (
	ctx        = context.Background()
	pool       *pgxpool.Pool
	client     *mongo.Client
	err        error
	backupList []string
)

type gfunc struct {
	param    string
	function func(p any) any
}

func getTag(field reflect.StructField) (json []string, bson []string) {
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

	fmt.Println(fieldType, cond)

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

func backupTool(schemaName string, schema any) {
	if len(backupList) != 0 && !slices.Contains(backupList, schemaName) {
		fmt.Println("Skipping backup of", schemaName)
		return
	}

	db := client.Database("infinity")

	cur, err := db.Collection(schemaName).Find(ctx, bson.M{})

	if err != nil {
		panic(err)
	}

	fmt.Println("Backing up " + schemaName)

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
		fmt.Println("Got tag of", tag, "for field ", field.Name)

		var (
			defaultVal = ""
			uniqueVal  = ""
		)

		if field.Tag.Get("unique") == "true" {
			uniqueVal = "UNIQUE"
		}

		if field.Tag.Get("default") != "" {
			defaultVal = field.Tag.Get("default")

			if strings.HasPrefix(defaultVal, "{}") {
				defaultVal = "'" + defaultVal + "'"
			}

			defaultVal = " DEFAULT " + defaultVal
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

			_, err := pool.Exec(ctx, "ALTER TABLE "+schemaName+" ADD CONSTRAINT "+tag[0]+"_fkey FOREIGN KEY ("+tag[0]+") REFERENCES "+fkeyRefersParentTable+"("+fkeyRefersParentColumn+")")

			if err != nil {
				panic(err)
			}
		}
	}

	var i int

	for cur.Next(ctx) {
		var result bson.M

		err := cur.Decode(&result)
		if err != nil {
			panic(err)
		}

		var sqlStr string = "INSERT INTO " + schemaName + " ("

		for _, field := range reflect.VisibleFields(structType) {
			tag, _ := getTag(field) // Json tag here again
			fmt.Println("Got tag of", tag, "for field ", field.Name)

			sqlStr += tag[0] + ","
		}

		sqlStr = sqlStr[:len(sqlStr)-1] + ") VALUES ("

		args := make([]any, 0)

		argNums := []string{}

		for i, field := range reflect.VisibleFields(structType) {
			tag, btag := getTag(field) // Here we need both
			fmt.Println("Got tag of", tag, "for field", field.Name, "with btag of", btag)

			var res any

			res = result[btag[0]]

			if res == nil {
				if field.Tag.Get("defaultfunc") != "" {
					fn := exportedFuncs[field.Tag.Get("defaultfunc")]
					if fn == nil {
						panic("Default function " + field.Tag.Get("defaultfunc") + " not found")
					}
					res = fn.function(result[fn.param])
				}
			}

			// We have to do this a second time after defaultfunc is called just in case it changed the value back to nil
			if res == nil {
				if field.Tag.Get("default") != "" {
					res = resolveInput(field.Tag.Get("default"))
				} else {
					// Ask user what to do
					var flag bool = true
					for flag {
						fmt.Println("Field", btag[0], "(", tag[0], ") is nil, what do you want to set this to? ")
						var input string
						fmt.Scanln(&input)
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

			if field.Tag.Get("log") == "1" {
				fmt.Println("Setting", btag[0], "(", tag[0], ") to", res)
			}

			// Handle mark of timestamptz
			if strings.HasPrefix(tag[1], "time") {
				// check if res is int64
				fmt.Println("Converting a", reflect.TypeOf(res), "to time.Time")
				if resCast, ok := res.(int64); ok {
					res = time.UnixMilli(resCast)
				} else if resCast, ok := res.(float64); ok {
					res = time.UnixMilli(int64(resCast))
				}
			}

			// Handle tolist
			if field.Tag.Get("tolist") == "true" {
				if resCast, ok := res.(string); ok {
					res = strings.Split(strings.ReplaceAll(resCast, " ", ""), ",")
					fmt.Println("Converting", resCast, "to", res)
				}
			}

			args = append(args, res)

			argNums = append(argNums, "$"+strconv.Itoa(i+1))
		}

		sqlStr += strings.Join(argNums, ",") + ")"

		fmt.Println("SQL String:", sqlStr)

		_, pgerr = pool.Exec(ctx, sqlStr, args...)

		if pgerr != nil {
			panic(pgerr)
		}

		i++

		fmt.Println("At", i, "rows")
	}
}

func main() {
	backupCols := flag.String("backup", "", "Which collections to backup. Default is all")

	flag.Parse()

	if *backupCols == "" {
		backupList = []string{}
	} else {
		backupList = strings.Split(*backupCols, ",")
	}

	if len(backupList) == 0 {
		fmt.Println("No collections specified, backing up all")
	}

	// Create mongodb conn
	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017/infinity"))

	if err != nil {
		panic(err)
	}

	// Create postgres conn
	pool, err = pgxpool.Connect(ctx, "postgresql://127.0.0.1:5432/merged?user=root&password=iblpublic")

	if err != nil {
		panic(err)
	}

	_, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

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
