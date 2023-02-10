package cli

import (
	"flag"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func init() { godotenv.Load() }

type SchemaOpts struct {
	TableName string
}

type App struct {
	SchemaOpts SchemaOpts
	BackupFunc func(source Source)
	LoadSource func(name string) (Source, error)
}

func Main(app App) {
	if app.LoadSource == nil {
		panic("cli: LoadSource is nil")
	}

	OnlySchema = flag.Bool("schema", false, "Only create schema")
	source := flag.String("source", "mongo", "Source to use. Must be listed in schemas.go")
	flag.Parse()

	if len(backupList) == 0 {
		NotifyMsg("info", "No specific rows specified, backing up all")
	}

	if *source == "" {
		NotifyMsg("error", "No source specified")
		return
	}

	dbSource, err := app.LoadSource(*source)

	if err != nil {
		NotifyMsg("error", "Failed to load source: "+err.Error())
		return
	}

	// Create postgres conn
	Pool, err = pgxpool.Connect(ctx, "postgresql:///"+app.SchemaOpts.TableName)

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

	app.BackupFunc(dbSource)

	Bar.Abort(true)

	Bar.Wait()
}
