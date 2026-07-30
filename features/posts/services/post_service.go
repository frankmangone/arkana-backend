package services

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/models"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type PostService struct {
	db            *sql.DB
	notifications *notifservices.NotificationService
}

func NewPostService(db *sql.DB, notifications *notifservices.NotificationService) *PostService {
	return &PostService{db: db, notifications: notifications}
}

// GetByPath finds a post by path_identifier.
func (s *PostService) GetByPath(path string) (*models.Post, error) {
	var p models.Post
	err := s.db.QueryRow(
		"SELECT id, path_identifier, like_count, created_at, updated_at FROM posts WHERE path_identifier = ?",
		path,
	).Scan(&p.ID, &p.PathIdentifier, &p.LikeCount, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetOrCreateByPath finds a post by path_identifier, creating it if it doesn't exist.
func (s *PostService) GetOrCreateByPath(path string) (*models.Post, error) {
	var p models.Post
	err := s.db.QueryRow(
		"SELECT id, path_identifier, like_count, created_at, updated_at FROM posts WHERE path_identifier = ?",
		path,
	).Scan(&p.ID, &p.PathIdentifier, &p.LikeCount, &p.CreatedAt, &p.UpdatedAt)
	if err == nil {
		return &p, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	result, err := s.db.Exec("INSERT INTO posts (path_identifier) VALUES (?)", path)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.getByID(int(id))
}

// MissingPaths returns the subset of paths that have no row in posts, for
// publish-time validation by other features (e.g. reading lists). Returns
// nil for an empty input without querying. Same IN (...) placeholder idiom
// as GetReadStatuses/tags.TagService.MissingTags.
func (s *PostService) MissingPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(paths))
	args := make([]interface{}, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}

	rows, err := s.db.Query(
		fmt.Sprintf("SELECT path_identifier FROM posts WHERE path_identifier IN (%s)", strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[string]bool, len(paths))
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		found[path] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var missing []string
	for _, p := range paths {
		if !found[p] {
			missing = append(missing, p)
		}
	}
	return missing, nil
}

// GetIDsByPaths returns a map of path_identifier -> posts.id for every
// path that has a matching row; paths with no match are simply absent.
// Callers that need to reject unknown paths call MissingPaths first —
// this method only resolves ids, never validates. Kept on PostService
// (rather than returning *models.Post) so a structural interface calling
// it never needs to import features/posts/models.
func (s *PostService) GetIDsByPaths(paths []string) (map[string]int, error) {
	if len(paths) == 0 {
		return map[string]int{}, nil
	}

	placeholders := make([]string, len(paths))
	args := make([]interface{}, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}

	rows, err := s.db.Query(
		fmt.Sprintf("SELECT id, path_identifier FROM posts WHERE path_identifier IN (%s)", strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int, len(paths))
	for rows.Next() {
		var id int
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return nil, err
		}
		result[path] = id
	}
	return result, rows.Err()
}

// ToggleLike adds or removes a like for the given user on the given post.
// On the unlike→like transition only, it notifies the post's writer (if
// set) as part of the same transaction.
func (s *PostService) ToggleLike(postID, userID int) (liked bool, likeCount int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRow(
		"SELECT 1 FROM post_likes WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Scan(&exists)

	if err == sql.ErrNoRows {
		_, err = tx.Exec(
			"INSERT INTO post_likes (post_id, user_id) VALUES (?, ?)",
			postID, userID,
		)
		if err != nil {
			return false, 0, err
		}
		_, err = tx.Exec("UPDATE posts SET like_count = like_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", postID)
		if err != nil {
			return false, 0, err
		}
		liked = true

		var writerUserID sql.NullInt64
		err = tx.QueryRow(
			"SELECT w.user_id FROM posts p JOIN writers w ON w.id = p.writer_id WHERE p.id = ?",
			postID,
		).Scan(&writerUserID)
		if err != nil && err != sql.ErrNoRows {
			return false, 0, err
		}
		if err == nil && writerUserID.Valid {
			if err := s.notifications.Create(tx, int(writerUserID.Int64), userID, notifmodels.TypePostLiked, &postID, nil); err != nil {
				return false, 0, err
			}
		}
	} else if err != nil {
		return false, 0, err
	} else {
		_, err = tx.Exec(
			"DELETE FROM post_likes WHERE post_id = ? AND user_id = ?",
			postID, userID,
		)
		if err != nil {
			return false, 0, err
		}
		_, err = tx.Exec("UPDATE posts SET like_count = like_count - 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", postID)
		if err != nil {
			return false, 0, err
		}
		liked = false
	}

	err = tx.QueryRow("SELECT like_count FROM posts WHERE id = ?", postID).Scan(&likeCount)
	if err != nil {
		return false, 0, err
	}

	if err := tx.Commit(); err != nil {
		return false, 0, err
	}

	return liked, likeCount, nil
}

// ToggleRead marks a post as read/unread for the given user. There is no
// counter to keep in sync (unlike ToggleLike's like_count) — read status has
// no public aggregate in this milestone.
func (s *PostService) ToggleRead(postID, userID int) (read bool, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRow(
		"SELECT 1 FROM post_reads WHERE post_id = ? AND user_id = ?",
		postID, userID,
	).Scan(&exists)

	if err == sql.ErrNoRows {
		_, err = tx.Exec(
			"INSERT INTO post_reads (post_id, user_id) VALUES (?, ?)",
			postID, userID,
		)
		if err != nil {
			return false, err
		}
		read = true
	} else if err != nil {
		return false, err
	} else {
		_, err = tx.Exec(
			"DELETE FROM post_reads WHERE post_id = ? AND user_id = ?",
			postID, userID,
		)
		if err != nil {
			return false, err
		}
		read = false
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}

	return read, nil
}

// GetReadStatuses returns path -> read status for the given user across many
// posts in one query, so a whole reading list's progress can be fetched in a
// single round trip instead of one request per article. Paths for posts that
// don't exist yet (never liked/read/commented on) default to false, same as
// paths the user simply hasn't read.
func (s *PostService) GetReadStatuses(paths []string, userID int) (map[string]bool, error) {
	result := make(map[string]bool, len(paths))
	for _, p := range paths {
		result[p] = false
	}

	if userID <= 0 || len(paths) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(paths))
	args := make([]interface{}, 0, len(paths)+1)
	for i, p := range paths {
		placeholders[i] = "?"
		args = append(args, p)
	}
	args = append(args, userID)

	query := fmt.Sprintf(
		`SELECT p.path_identifier
		 FROM posts p
		 JOIN post_reads r ON r.post_id = p.id
		 WHERE p.path_identifier IN (%s) AND r.user_id = ?`,
		strings.Join(placeholders, ","),
	)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		result[path] = true
	}

	return result, rows.Err()
}

func (s *PostService) getByID(id int) (*models.Post, error) {
	var p models.Post
	err := s.db.QueryRow(
		"SELECT id, path_identifier, like_count, created_at, updated_at FROM posts WHERE id = ?",
		id,
	).Scan(&p.ID, &p.PathIdentifier, &p.LikeCount, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

var ErrPostNotFound = errors.New("post not found")

// GetPostInfo returns post info by path, including whether a specific user has
// liked and read it. If userID is 0, liked and read are always false.
func (s *PostService) GetPostInfo(path string, userID int) (*models.PostInfoResponse, error) {
	var likeCount int
	var postID int

	err := s.db.QueryRow(
		"SELECT id, like_count FROM posts WHERE path_identifier = ?",
		path,
	).Scan(&postID, &likeCount)

	if err == sql.ErrNoRows {
		return nil, ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}

	var liked, read bool
	if userID > 0 {
		var exists int
		err = s.db.QueryRow(
			"SELECT 1 FROM post_likes WHERE post_id = ? AND user_id = ?",
			postID, userID,
		).Scan(&exists)

		if err == nil {
			liked = true
		} else if err != sql.ErrNoRows {
			return nil, err
		}

		err = s.db.QueryRow(
			"SELECT 1 FROM post_reads WHERE post_id = ? AND user_id = ?",
			postID, userID,
		).Scan(&exists)

		if err == nil {
			read = true
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}

	return &models.PostInfoResponse{
		Path:      path,
		LikeCount: likeCount,
		Liked:     liked,
		Read:      read,
	}, nil
}

// ListVisibleContentPage returns one page of visible post_contents rows
// (ordered by id for stable pagination across pages) plus the total
// visible count, for the admin-authenticated CI content pull. Unlike
// Publish's per-post_id upserts, this reads across all posts and
// languages at once.
func (s *PostService) ListVisibleContentPage(limit, offset int) ([]models.AdminPostContentItem, int, error) {
	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM post_contents WHERE visible = 1").Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(
		"SELECT lang, path, content FROM post_contents WHERE visible = 1 ORDER BY id ASC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []models.AdminPostContentItem{}
	for rows.Next() {
		var item models.AdminPostContentItem
		if err := rows.Scan(&item.Lang, &item.Path, &item.Content); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}
