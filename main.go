// You should not need to edit this file unless you need to debug something
package main

import (
	"context"
	"flag"
	"fmt"
	"hepatitis-antiviral/cli"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

var (
	ctx        = context.Background()
	backupList []string
)

type schemaOpts struct {
	TableName string
}

func init() { godotenv.Load() }

func main() {
	backupCols := flag.String("backup", "", "Which collections to backup. Default is all")
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

	schemaOpts := getOpts()

	// Create postgres conn
	cli.Pool, err = pgxpool.Connect(ctx, "postgresql://127.0.0.1:5432/"+schemaOpts.TableName+"?user=root&password=iblpublic")

	if err != nil {
		panic(err)
	}

	_, err = cli.Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	if err != nil {
		panic(err)
	}

	if len(backupList) == 0 {
		cli.Pool.Exec(ctx, `DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO public;
COMMENT ON SCHEMA public IS 'standard public schema'`)
		cli.Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
	}

	backupSchemas()
}
