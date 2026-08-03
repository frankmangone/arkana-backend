package db

import "database/sql"

// Transact runs fn inside a transaction opened on sqlDB: fn's transaction
// is committed if it returns nil, and rolled back otherwise. It exists to
// collapse the repeated Begin/defer Rollback/Commit boilerplate that every
// transactional service method needs, into one call.
//
// Named Transact (not WithTx) to avoid confusion with the WithTx method
// each module's Queries interface already has, which does something
// different: it rebinds a Queries value to run against a given *sql.Tx
// instead of the module's plain *sql.DB. Transact is what opens that Tx in
// the first place; a service method typically combines both:
//
//	err := db.Transact(s.db, func(tx *sql.Tx) error {
//	    qtx := s.queries.WithTx(tx)
//	    return qtx.DoSomething()
//	})
func Transact(sqlDB *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
