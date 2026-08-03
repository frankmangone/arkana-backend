package db

import "database/sql"

// DBTX is satisfied by both *sql.DB and *sql.Tx, letting query methods run
// against either a plain connection or an open transaction.
type DBTX interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
