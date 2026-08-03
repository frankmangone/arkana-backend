package db

import "strings"

// Placeholders and ToAny together build a dynamic SQL "IN (...)" clause
// from a Go slice of unknown length - something several query methods
// across this codebase need, since database/sql has no native way to
// bind a slice to a single "?" placeholder.
//
// Full example, looking up rows whose id is in a slice of ids:
//
//	ids := []int{4, 8, 15}
//
//	query := fmt.Sprintf("SELECT name FROM widgets WHERE id IN (%s)", db.Placeholders(len(ids)))
//	// query is now: "SELECT name FROM widgets WHERE id IN (?,?,?)"
//
//	rows, err := conn.Query(query, db.ToAny(ids)...)
//	// db.ToAny(ids) is []any{4, 8, 15} - the args Query needs to fill in
//	// those three placeholders, one per id.
//
// If the query needs extra fixed arguments before or after the IN clause's
// own placeholders (e.g. "WHERE id IN (...) AND user_id = ?"), append them
// onto ToAny's result in the same order they appear in the query string:
//
//	rows, err := conn.Query(query+" AND user_id = ?", append(db.ToAny(ids), userID)...)
//
// Placeholders returns a comma-separated string of n "?" placeholders
// ("?,?,?" for n=3). Returns "" for n <= 0.
func Placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// ToAny converts a typed slice into []any, so it can be passed as the
// variadic args to database/sql's Query/QueryRow/Exec - which only accept
// ...any, never a typed slice directly (even []string doesn't satisfy
// ...any without this conversion).
func ToAny[T any](items []T) []any {
	args := make([]any, len(items))
	for i, v := range items {
		args[i] = v
	}
	return args
}
