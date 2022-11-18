package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func init() { godotenv.Load() }

type SchemaOpts struct {
	TableName string
}

type App struct {
	SchemaOpts SchemaOpts
	BackupFunc func()
}

func Main(app App) {
	backupCols := flag.String("backup", "", "Which collections to copy. Default is all")
	flag.Parse()

	if *backupCols == "" {
		backupList = []string{}
	} else {
		backupList = strings.Split(*backupCols, ",")
	}

	if len(backupList) == 0 {
		fmt.Println("No collections specified, backing up all")
	}

	// Create postgres conn
	var err error
	Pool, err = pgxpool.Connect(ctx, "postgresql://127.0.0.1:5432/"+app.SchemaOpts.TableName+"?user=root&password=iblpublic")

	if err != nil {
		panic(err)
	}

	_, err = Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	if err != nil {
		panic(err)
	}

	if len(backupList) == 0 {
		Pool.Exec(ctx, `DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO postgres;
GRANT ALL ON SCHEMA public TO public;
COMMENT ON SCHEMA public IS 'standard public schema'`)
		Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
	}

	app.BackupFunc()

	Bar.Abort(true)

	Bar.Wait()
}
