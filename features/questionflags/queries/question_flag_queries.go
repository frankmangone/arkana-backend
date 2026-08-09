package queries

import (
	"arkana/features/questionflags/models"
	dbpkg "arkana/shared/db"
)

type QuestionFlagQueries interface {
	// ResolveQuestionID maps a question's public uuid to its internal id.
	// Returns sql.ErrNoRows if no question has that uuid.
	ResolveQuestionID(uuid string) (int, error)
	// Upsert records a flag from userID on questionID, or overwrites the
	// reason (and bumps created_at) if that user already flagged this
	// question - one flag per user per question, not one per submission.
	Upsert(questionID, userID int, reason string) (*models.QuestionFlag, error)
	// List returns every flag, newest first, joined with enough question/
	// user context to act on without a second lookup.
	List() ([]models.QuestionFlag, error)
	// DeleteAll removes every flag, returning how many rows were removed.
	DeleteAll() (int64, error)
	// Delete removes a single flag by id, returning how many rows were
	// removed (0 if no flag had that id).
	Delete(id int) (int64, error)
}

type SQLQuestionFlagQueries struct {
	db dbpkg.DBTX
}

func NewSQLQuestionFlagQueries(db dbpkg.DBTX) *SQLQuestionFlagQueries {
	return &SQLQuestionFlagQueries{db: db}
}

func (q *SQLQuestionFlagQueries) ResolveQuestionID(uuid string) (int, error) {
	var id int
	err := q.db.QueryRow("SELECT id FROM questions WHERE uuid = ?", uuid).Scan(&id)
	return id, err
}

func (q *SQLQuestionFlagQueries) Upsert(questionID, userID int, reason string) (*models.QuestionFlag, error) {
	if _, err := q.db.Exec(
		`INSERT INTO question_flags (question_id, user_id, reason) VALUES (?, ?, ?)
		 ON CONFLICT(question_id, user_id) DO UPDATE SET reason = excluded.reason, created_at = CURRENT_TIMESTAMP`,
		questionID, userID, reason,
	); err != nil {
		return nil, err
	}

	var f models.QuestionFlag
	err := q.db.QueryRow(
		`SELECT id, question_id, user_id, reason, created_at FROM question_flags WHERE question_id = ? AND user_id = ?`,
		questionID, userID,
	).Scan(&f.ID, &f.QuestionID, &f.UserID, &f.Reason, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (q *SQLQuestionFlagQueries) List() ([]models.QuestionFlag, error) {
	rows, err := q.db.Query(
		`SELECT qf.id, qf.question_id, q.uuid, q.slug, qf.user_id, u.email, qf.reason, qf.created_at
		 FROM question_flags qf
		 JOIN questions q ON q.id = qf.question_id
		 JOIN users u ON u.id = qf.user_id
		 ORDER BY qf.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	flags := []models.QuestionFlag{}
	for rows.Next() {
		var f models.QuestionFlag
		if err := rows.Scan(
			&f.ID, &f.QuestionID, &f.QuestionUUID, &f.QuestionSlug, &f.UserID, &f.UserEmail, &f.Reason, &f.CreatedAt,
		); err != nil {
			return nil, err
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return flags, nil
}

func (q *SQLQuestionFlagQueries) DeleteAll() (int64, error) {
	result, err := q.db.Exec("DELETE FROM question_flags")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (q *SQLQuestionFlagQueries) Delete(id int) (int64, error) {
	result, err := q.db.Exec("DELETE FROM question_flags WHERE id = ?", id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
