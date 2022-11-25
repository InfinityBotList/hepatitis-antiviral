package migrations

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TableExists(ctx context.Context, pool *pgxpool.Pool, name string) bool {
	var exists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", name).Scan(&exists)

	if err != nil {
		panic(err)
	}

	return exists
}

func ColExists(ctx context.Context, pool *pgxpool.Pool, table, col string) bool {
	var exists bool
	err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)", table, col).Scan(&exists)

	if err != nil {
		panic(err)
	}

	return exists
}

type migrator struct {
	name string
	fn   func(context.Context, *pgxpool.Pool)
}
