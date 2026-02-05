package services

import (
	"arkana/features/posts/models"
	"database/sql"
	"errors"
)

type PostService struct {
	db *sql.DB
}

func NewPostService(db *sql.DB) *PostService {
	return &PostService{db: db}
}

// GetByPath finds a post by path_identifier.
// Returns ErrPostNotFound if the post doesn't exist.
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

// ToggleLike adds or removes a like for the given wallet on the given post.
// Returns whether the post is now liked and the new like count.
func (s *PostService) ToggleLike(postID, walletID int) (liked bool, likeCount int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	// Check if already liked
	var exists int
	err = tx.QueryRow(
		"SELECT 1 FROM post_likes WHERE post_id = ? AND wallet_id = ?",
		postID, walletID,
	).Scan(&exists)

	if err == sql.ErrNoRows {
		// Not liked yet — add like
		_, err = tx.Exec(
			"INSERT INTO post_likes (post_id, wallet_id) VALUES (?, ?)",
			postID, walletID,
		)
		if err != nil {
			return false, 0, err
		}
		_, err = tx.Exec("UPDATE posts SET like_count = like_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?", postID)
		if err != nil {
			return false, 0, err
		}
		liked = true
	} else if err != nil {
		return false, 0, err
	} else {
		// Already liked — remove
		_, err = tx.Exec(
			"DELETE FROM post_likes WHERE post_id = ? AND wallet_id = ?",
			postID, walletID,
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

	// Read back the current count
	err = tx.QueryRow("SELECT like_count FROM posts WHERE id = ?", postID).Scan(&likeCount)
	if err != nil {
		return false, 0, err
	}

	if err := tx.Commit(); err != nil {
		return false, 0, err
	}

	return liked, likeCount, nil
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

// GetPostInfo returns post info by path, including whether a specific wallet has liked it.
// If walletAddress is empty, liked will always be false.
// Returns ErrPostNotFound if the post doesn't exist.
func (s *PostService) GetPostInfo(path string, walletAddress string) (*models.PostInfoResponse, error) {
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

	// Check if wallet has liked this post
	var liked bool
	if walletAddress != "" {
		var exists int
		err = s.db.QueryRow(`
			SELECT 1 FROM post_likes pl
			JOIN wallets w ON w.id = pl.wallet_id
			WHERE pl.post_id = ? AND LOWER(w.address) = LOWER(?)
		`, postID, walletAddress).Scan(&exists)

		if err == nil {
			liked = true
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}

	return &models.PostInfoResponse{
		Path:      path,
		LikeCount: likeCount,
		Liked:     liked,
	}, nil
}
