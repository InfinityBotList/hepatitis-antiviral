// You should not need to edit this file unless you need to debug something
package cli

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vbauerster/mpb/v8"
	"golang.org/x/exp/slices"
)

var (
	ctx        = context.Background()
	Pool       *pgxpool.Pool
	backupList []string
	tagCache   map[string][2][]string = make(map[string][2][]string)

	Map        map[string]any
	OnlySchema *bool
)

type ExportedFunction struct {
	Param    string
	Function func(p any) any
}

type BackupOpts struct {
	Debug             bool
	IgnoreFKError     bool
	IgnoreUniqueError bool
	RenameTo          string
	IndexCols         []string
	ExportedFuncs     map[string]*ExportedFunction
}

type Source interface {
	// Returns the records of a entity (collection in mongo, row in postgres etc)
	GetRecords(entity string) ([]map[string]any, error)
	// Gets the count of records in a entity
	GetCount(entity string) (int64, error)
	// Extra parsers
	ExtParse(res any) (any, error)
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

func BackupTool(source Source, schemaName string, schema any, opts BackupOpts) {
	if Bar == nil {
		mb = mpb.New(mpb.WithWidth(64))
	}

	if opts.ExportedFuncs == nil {
		panic("exportedFuncs cannot be nil")
	}

	tagCache = make(map[string][2][]string)

	if len(backupList) != 0 && !slices.Contains(backupList, schemaName) {
		NotifyMsg("info", "Skipping backup of "+schemaName)
		return
	}

	var err error
	var cerr error

	if len(backupList) != 0 {
		// Try deleting but ignore if delete fails
		_, err = Pool.Exec(ctx, "DROP TABLE "+schemaName)

		if err != nil {
			NotifyMsg("error", "Failed to drop table "+schemaName+": "+err.Error())
		}
	}

	_, pgerr := Pool.Exec(ctx, "CREATE TABLE "+schemaName+" (itag UUID PRIMARY KEY NOT NULL DEFAULT uuid_generate_v4())")

	if pgerr != nil {
		panic(pgerr)
	}

	structType := reflect.TypeOf(schema)

	// Schema generation
	for _, field := range reflect.VisibleFields(structType) {
		tag, _ := getTag(field) // We want json tag here as it has what we need
		NotifyMsg("debug", fmt.Sprintln("Got tag of", tag, "for field ", field.Name))

		var (
			defaultVal = ""
			uniqueVal  = ""
		)

		if field.Tag.Get("unique") == "true" {
			NotifyMsg("debug", fmt.Sprintln("Field", field.Name, "is unique"))
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
		_, err := Pool.Exec(ctx, "ALTER TABLE "+schemaName+" ADD COLUMN "+tag[0]+" "+strings.Join(tag[1:], " ")+uniqueVal+defaultVal)
		if err != nil {
			NotifyMsg("error", "ALTER TABLE "+schemaName+" ADD COLUMN "+tag[0]+" "+strings.Join(tag[1:], " ")+uniqueVal+defaultVal)
			panic(err)
		}

		// Check for fkey, if so add it
		if field.Tag.Get("fkey") != "" {
			// Format for fkey is REFER_TABLE_NAME,COLUMN_NAME
			fkeySplit := strings.Split(field.Tag.Get("fkey"), ",")
			fkeyRefersParentTable := fkeySplit[0]
			fkeyRefersParentColumn := fkeySplit[1]

			_, err := Pool.Exec(ctx, "ALTER TABLE "+schemaName+" ADD CONSTRAINT "+tag[0]+"_fkey FOREIGN KEY ("+tag[0]+") REFERENCES "+fkeyRefersParentTable+"("+fkeyRefersParentColumn+") ON DELETE CASCADE ON UPDATE CASCADE")

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

		_, pgerr = Pool.Exec(ctx, sqlStr)

		if pgerr != nil {
			panic(pgerr)
		}
	}

	// If only schema, exit here
	if *OnlySchema {
		return
	}

	data, err := source.GetRecords(schemaName)

	if err != nil {
		panic(err)
	}

	count, cerr := source.GetCount(schemaName)

	if cerr != nil {
		panic(cerr)
	}

	var counter int

	StartBar(schemaName, count, true)
	for _, result := range data {
		if counter == 0 {
			NotifyMsg("info", "Backing up "+schemaName)
		}

		counter++

		Map = result

		Bar.Increment()

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
			if opts.Debug {
				NotifyMsg("debug", "Table:"+schemaName+"\nField:"+field.Name+"\nType:"+tag[1]+"\n")
			}

			var res any

			res = result[btag[0]]

			if res == "" {
				res = nil
			}

			if res == nil {
				if field.Tag.Get("defaultfunc") != "" {
					fn := opts.ExportedFuncs[field.Tag.Get("defaultfunc")]
					if fn == nil {
						panic("Default function " + field.Tag.Get("defaultfunc") + " not found")
					}
					res = fn.Function(result[fn.Param])
				}
			}

			if field.Tag.Get("pre") != "" {
				fn := opts.ExportedFuncs[field.Tag.Get("pre")]
				if fn == nil {
					panic("Pre function " + field.Tag.Get("pre") + " not found")
				}
				res = fn.Function(result[fn.Param])
			}

			// We have to do this a second time after defaultfunc is called just in case it changed the value back to nil
			if res == nil {
				if field.Tag.Get("default") != "" {
					if strings.Contains(field.Tag.Get("default"), "SKIP") {
						NotifyMsg("warning", "Skipping row due to default value at iteration "+strconv.Itoa(counter))
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
					var msg = PromptServerChannel("What should the value of " + tag[0] + " be? (currently null)")

					res = resolveInput(msg)
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
				if opts.Debug {
					NotifyMsg("debug", "Converting a "+reflect.TypeOf(res).Name()+" to time.Time")
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
				}
			}

			result, err := source.ExtParse(res)

			if err == nil {
				res = result
			}

			// Handle tolist
			if field.Tag.Get("tolist") == "true" {
				if resCast, ok := res.(string); ok {
					res = strings.Split(strings.ReplaceAll(resCast, " ", ""), ",")

					if opts.Debug {
						NotifyMsg("debug", "Converting "+resCast+" to list")
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

		if opts.Debug {
			NotifyMsg("debug", "SQL String: "+sqlStr)
		}

		_, pgerr = Pool.Exec(ctx, sqlStr, args...)

		if pgerr != nil {
			if opts.IgnoreFKError && strings.Contains(pgerr.Error(), "violates foreign key") {
				NotifyMsg("warning", "Ignoring foreign key error on iter "+strconv.Itoa(counter)+": "+pgerr.Error())
				continue
			} else if opts.IgnoreUniqueError && strings.Contains(pgerr.Error(), "unique constraint") {
				NotifyMsg("warning", "Ignoring unique error on iter "+strconv.Itoa(counter)+": "+pgerr.Error())
				continue
			}
			NotifyMsg("error", "Error on iter "+strconv.Itoa(counter)+": "+pgerr.Error())
			NotifyMsg("error", "Failing SQL: "+sqlStr+"\nArgs: "+fmt.Sprint(args))
			fmt.Println("Failing SQL: ", sqlStr, args)
			for _, arg := range args {
				if arg != nil {
					fmt.Println(reflect.TypeOf(arg), arg)
				}
			}
			panic(pgerr)
		}
	}

	if opts.RenameTo != "" {
		// Rename postgres table
		sqlStr := "ALTER TABLE " + schemaName + " RENAME TO " + opts.RenameTo
		_, pgerr = Pool.Exec(ctx, sqlStr)

		if pgerr != nil {
			panic(pgerr)
		}
	}
}
