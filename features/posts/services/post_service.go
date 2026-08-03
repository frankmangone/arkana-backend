package services

import (
	notifmodels "arkana/features/notifications/models"
	notifservices "arkana/features/notifications/services"
	"arkana/features/posts/models"
	"arkana/features/posts/queries"
	dbpkg "arkana/shared/db"
	"database/sql"
	"errors"
)

var ErrPostNotFound = errors.New("post not found")

type PostService struct {
	db            *sql.DB
	queries       queries.PostQueries
	notifications *notifservices.NotificationService
}

func NewPostService(db *sql.DB, notifications *notifservices.NotificationService) *PostService {
	return &PostService{db: db, queries: queries.NewSQLPostQueries(db), notifications: notifications}
}

// GetByPath finds a post by path_identifier.
func (s *PostService) GetByPath(path string) (*models.Post, error) {
	p, err := s.queries.GetByPath(path)
	if err == sql.ErrNoRows {
		return nil, ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetOrCreateByPath finds a post by path_identifier, creating it if it doesn't exist.
func (s *PostService) GetOrCreateByPath(path string) (*models.Post, error) {
	p, err := s.queries.GetByPath(path)
	if err == nil {
		return p, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	id, err := s.queries.InsertPost(path)
	if err != nil {
		return nil, err
	}

	return s.queries.GetByID(int(id))
}

// MissingPaths returns the subset of paths that have no row in posts, for
// publish-time validation by other features.
func (s *PostService) MissingPaths(paths []string) ([]string, error) {
	return s.queries.MissingPaths(paths)
}

// GetIDsByPaths returns a map of path_identifier -> posts.id for every
// path that has a matching row.
func (s *PostService) GetIDsByPaths(paths []string) (map[string]int, error) {
	return s.queries.GetIDsByPaths(paths)
}

// ToggleLike adds or removes a like for the given user on the given post.
// On the unlike->like transition only, it notifies the post's writer (if
// set) as part of the same transaction.
func (s *PostService) ToggleLike(postID, userID int) (bool, int, error) {
	var liked bool
	var likeCount int
	err := dbpkg.Transact(s.db, func(tx *sql.Tx) error {
		qtx := s.queries.WithTx(tx)

		exists, err := qtx.LikeExists(postID, userID)
		if err != nil {
			return err
		}

		if !exists {
			if err := qtx.InsertLike(postID, userID); err != nil {
				return err
			}
			if err := qtx.IncrementLikeCount(postID); err != nil {
				return err
			}
			liked = true

			writerUserID, err := qtx.GetPostWriterUserID(postID)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if err == nil && writerUserID.Valid {
				if err := s.notifications.Create(tx, int(writerUserID.Int64), userID, notifmodels.TypePostLiked, &postID, nil); err != nil {
					return err
				}
			}
		} else {
			if err := qtx.DeleteLike(postID, userID); err != nil {
				return err
			}
			if err := qtx.DecrementLikeCount(postID); err != nil {
				return err
			}
			liked = false
		}

		count, err := qtx.GetLikeCount(postID)
		if err != nil {
			return err
		}
		likeCount = count
		return nil
	})
	if err != nil {
		return false, 0, err
	}

	return liked, likeCount, nil
}

// ToggleRead marks a post as read/unread for the given user.
func (s *PostService) ToggleRead(postID, userID int) (bool, error) {
	var read bool
	err := dbpkg.Transact(s.db, func(tx *sql.Tx) error {
		qtx := s.queries.WithTx(tx)

		exists, err := qtx.ReadExists(postID, userID)
		if err != nil {
			return err
		}

		if !exists {
			if err := qtx.InsertRead(postID, userID); err != nil {
				return err
			}
			read = true
		} else {
			if err := qtx.DeleteRead(postID, userID); err != nil {
				return err
			}
			read = false
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	return read, nil
}

// GetReadStatuses returns path -> read status for the given user across
// many posts in one query.
func (s *PostService) GetReadStatuses(paths []string, userID int) (map[string]bool, error) {
	return s.queries.GetReadStatuses(paths, userID)
}

// GetPostInfo returns post info by path, including whether a specific user
// has liked and read it. If userID is 0, liked and read are always false.
func (s *PostService) GetPostInfo(path string, userID int) (*models.PostInfoResponse, error) {
	postID, likeCount, err := s.queries.GetPostInfo(path)
	if err == sql.ErrNoRows {
		return nil, ErrPostNotFound
	}
	if err != nil {
		return nil, err
	}

	var liked, read bool
	if userID > 0 {
		liked, err = s.queries.UserHasLiked(postID, userID)
		if err != nil {
			return nil, err
		}
		read, err = s.queries.UserHasRead(postID, userID)
		if err != nil {
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
// plus the total visible count, for the admin-authenticated CI content pull.
func (s *PostService) ListVisibleContentPage(limit, offset int) ([]models.AdminPostContentItem, int, error) {
	return s.queries.ListVisibleContentPage(limit, offset)
}
