// Package postgres defines a postgres backup source for hepatitis-antiviral
// Implements only BackupSource for now
package postgres

import (
	"context"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	URL  string
	conn *pgxpool.Pool
}

func (m *PostgresStore) Connect() error {
	// Connect to postgres
	conn, err := pgxpool.New(context.Background(), m.URL)

	if err != nil {
		return err
	}

	m.conn = conn
	return nil
}

func (m PostgresStore) GetRecords(entity string) ([]map[string]any, error) {
	var records []map[string]any

	rows, err := m.conn.Query(context.Background(), "SELECT * FROM "+entity)

	if err != nil {
		return nil, err
	}

	err = pgxscan.ScanAll(&records, rows)

	if err != nil {
		return nil, err
	}

	rows.Close()

	return records, nil
}

func (m PostgresStore) GetCount(entity string) (int64, error) {
	var count int64

	err := m.conn.QueryRow(context.Background(), "SELECT count(*) FROM "+entity).Scan(&count)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func (m PostgresStore) RecordList() ([]string, error) {
	rows, err := m.conn.Query(context.Background(), "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname != 'pg_catalog' AND schemaname != 'information_schema'")

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var records []string

	for rows.Next() {
		var record string
		err := rows.Scan(&record)

		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}

	return records, nil
}

// No special types for postgres, just use the default
func (m PostgresStore) ExtParse(res any) (any, error) {
	return res, nil
}
