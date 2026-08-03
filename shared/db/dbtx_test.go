package db

import (
	"database/sql"
	"testing"
)

func TestDBTXSatisfiedBySQLDBAndTx(t *testing.T) {
	var _ DBTX = (*sql.DB)(nil)
	var _ DBTX = (*sql.Tx)(nil)
}
