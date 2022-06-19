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

// Schemas here
//
// Either use schema struct tag (or bson + mark struct tag for special type overrides)

type Auth struct {
	ID    *string `bson:"id,omitempty" schema:"id,text not null"`
	Token string  `bson:"token" schema:"token,text not null"`
}

type UUID = string

type Bot struct {
	BotID            string    `bson:"botID" json:"bot_id" mark:"text not null unique"`
	Name             string    `bson:"botName" json:"name"`
	TagsRaw          string    `bson:"tags" json:"tags" tolist:"true"`
	Prefix           *string   `bson:"prefix" json:"prefix"`
	Owner            string    `bson:"main_owner" json:"owner" log:"1"`
	AdditionalOwners []string  `bson:"additional_owners" json:"additional_owners"`
	StaffBot         bool      `bson:"staff" json:"staff_bot" default:"false"`
	Short            string    `bson:"short" json:"short"`
	Long             string    `bson:"long" json:"long"`
	Library          *string   `bson:"library,omitempty" json:"library" default:"null"`
	Website          *string   `bson:"website,omitempty" json:"website" default:"null"`
	Donate           *string   `bson:"donate,omitempty" json:"donate" default:"null"`
	Support          *string   `bson:"support,omitempty" json:"support" default:"null"`
	NSFW             bool      `bson:"nsfw" json:"nsfw" default:"false"`
	Premium          bool      `bson:"premium" json:"premium" default:"false"`
	Certified        bool      `bson:"certified" json:"certified" default:"false"`
	PendingCert      bool      `bson:"pending_cert" json:"pending_cert" default:"false"`
	Servers          int       `bson:"servers" json:"servers" default:"0"`
	Shards           int       `bson:"shards" json:"shards" default:"0"`
	Users            int       `bson:"users" json:"users" default:"0"`
	ShardSet         []int     `bson:"shardArray" json:"shard_list" default:"{}"`
	Votes            int       `bson:"votes" json:"votes" default:"0"`
	Clicks           int       `bson:"clicks" json:"clicks" default:"0"`
	InviteClicks     int       `bson:"invite_clicks" json:"invite_clicks" default:"0"`
	Github           *string   `bson:"github,omitempty" json:"github" default:"null"`
	Banner           *string   `bson:"background,omitempty" json:"banner" default:"null"`
	Invite           *string   `bson:"invite" json:"invite" default:"null"`
	Type             string    `bson:"type" json:"type"`
	Vanity           *string   `bson:"vanity,omitempty" json:"vanity" default:"null"`
	ExternalSource   string    `bson:"external_source,omitempty" json:"external_source" default:"null"`
	ListSource       string    `bson:"listSource,omitempty" json:"list_source" mark:"uuid" default:"null"`
	VoteBanned       bool      `bson:"vote_banned,omitempty" json:"vote_banned" default:"false"`
	CrossAdd         bool      `bson:"cross_add,omitempty" json:"cross_add" default:"true"`
	StartPeriod      int64     `bson:"start_period,omitempty" json:"start_premium_period" default:"0"`
	SubPeriod        int64     `bson:"sub_period,omitempty" json:"premium_period_length" default:"0"`
	CertReason       string    `bson:"cert_reason,omitempty" json:"cert_reason" default:"null"`
	Announce         bool      `bson:"announce,omitempty" json:"announce" default:"false"`
	AnnounceMessage  string    `bson:"announce_msg,omitempty" json:"announce_message" default:"null"`
	Uptime           int64     `bson:"uptime,omitempty" json:"uptime" default:"0"`
	TotalUptime      int64     `bson:"total_uptime,omitempty" json:"total_uptime" default:"0"`
	Claimed          bool      `bson:"claimed,omitempty" json:"claimed" default:"false"`
	ClaimedBy        string    `bson:"claimedBy,omitempty" json:"claimed_by" default:"null"`
	Note             string    `bson:"note,omitempty" json:"approval_note" default:"No note"`
	Date             time.Time `bson:"date,omitempty" json:"date" mark:"timestamptz" schema:"date,timestamptz not null default NOW()"`
	Webhook          *string   `bson:"webhook,omitempty" json:"webhook" default:"null"` // Discord
	WebAuth          *string   `bson:"webAuth,omitempty" json:"web_auth" default:"null"`
	WebURL           *string   `bson:"webURL,omitempty" json:"custom_webhook" default:"null"`
	UniqueClicks     []string  `bson:"unique_clicks,omitempty" json:"unique_clicks" default:"{}"`
	Token            string    `bson:"token,omitempty" json:"token" default:"uuid_generate_v4()::text"`
}

// Tool below

func getTag(field reflect.StructField) (json []string, bson []string) {
	var tagSplit []string
	if field.Tag.Get("schema") != "" {
		tagSplit = strings.Split(field.Tag.Get("schema"), ",")
	} else {
		// Try fetching from bson and struct type instead
		if field.Tag.Get("bson") != "" {
			tagSplit = strings.Split(field.Tag.Get("bson"), ",")
			jsonKeyName := strings.Split(field.Tag.Get("json"), ",")

			if len(jsonKeyName) < 1 {
				panic("No json key name found for bson tag")
			}

			if jsonKeyName[0] == "-" {
				jsonKeyName[0] = tagSplit[0]
			}

			if jsonKeyName[0] == "" || jsonKeyName[0] == "-" {
				panic("No json key name found for bson tag")
			}

			var cond string

			if len(tagSplit) == 1 {
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

			if field.Type.Kind() == reflect.Slice || field.Tag.Get("tolist") == "true" {
				fieldType += "[]"
			}

			if field.Tag.Get("mark") != "" {
				fieldType = field.Tag.Get("mark")
			}

			fmt.Println(fieldType)

			return []string{jsonKeyName[0], fieldType + " " + cond}, []string{tagSplit[0], fieldType + " " + cond}
		} else {
			panic("No tag found for field " + field.Name)
		}
	}
	return tagSplit, tagSplit
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

	fmt.Println("Backing up " + schemaName)

	// Create new table
	pool.Exec(ctx, "DROP TABLE "+schemaName)
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

		// Create column
		_, err := pool.Exec(ctx, "ALTER TABLE "+schemaName+" ADD COLUMN "+tag[0]+" "+strings.Join(tag[1:], " "))
		if err != nil {
			panic(err)
		}
	}

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
			if field.Tag.Get("mark") == "timestamptz" {
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

	if len(backupList) == 0 || slices.Contains(backupList, "oauths") {
		backupTool("oauths", Auth{})
	}
	backupTool("bots", Bot{})
}
