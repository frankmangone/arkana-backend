package services

import (
	"database/sql"
	"fmt"
)

// PostIndexer indexes a post into the search backend. Satisfied
// structurally by *search/services.SearchService, without this package
// depending on it directly.
type PostIndexer interface {
	IndexPost(lang, path, title, description, content string, tags []string) error
}

// PublishInput is the raw content for one (post, language) pair, coming
// straight from a CI publish workflow with no pre-processing - frontmatter
// parsing and search-text stripping both happen server-side in Publish, so
// there's a single place that understands the content format.
type PublishInput struct {
	Path       string
	Lang       string
	RawContent string
}

// AdminPostService handles admin/CI-driven content publishing, distinct
// from PostService's read/like/comment responsibilities. It composes the
// existing PostService rather than duplicating posts-row logic.
type AdminPostService struct {
	db      *sql.DB
	posts   *PostService
	indexer PostIndexer
}

func NewAdminPostService(db *sql.DB, posts *PostService, indexer PostIndexer) *AdminPostService {
	return &AdminPostService{db: db, posts: posts, indexer: indexer}
}

// Publish parses RawContent's frontmatter, upserts the posts/post_contents
// rows for one (path, lang) with the body, and indexes a search-stripped
// version into search. Re-publishing the same path/lang updates the
// existing row rather than duplicating it.
func (s *AdminPostService) Publish(input PublishInput) error {
	frontmatter, body, err := parseFrontmatter(input.RawContent)
	if err != nil {
		return fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	title := frontmatterString(frontmatter, "title")
	thumbnail := frontmatterString(frontmatter, "thumbnail")
	description := frontmatterString(frontmatter, "description")
	tags := frontmatterStringSlice(frontmatter, "tags")

	post, err := s.posts.GetOrCreateByPath(input.Path)
	if err != nil {
		return err
	}

	contentPath := input.Path + ".md"

	var titleCol, thumbnailCol sql.NullString
	if title != "" {
		titleCol = sql.NullString{String: title, Valid: true}
	}
	if thumbnail != "" {
		thumbnailCol = sql.NullString{String: thumbnail, Valid: true}
	}

	_, err = s.db.Exec(
		`INSERT INTO post_contents (post_id, lang, path, content, title, thumbnail, visible)
		 VALUES (?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT (lang, path) DO UPDATE SET
		   post_id = excluded.post_id, content = excluded.content,
		   title = excluded.title, thumbnail = excluded.thumbnail,
		   visible = excluded.visible, updated_at = CURRENT_TIMESTAMP`,
		post.ID, input.Lang, contentPath, body, titleCol, thumbnailCol,
	)
	if err != nil {
		return err
	}

	searchContent := stripMarkdownForSearch(body)

	return s.indexer.IndexPost(input.Lang, input.Path, title, description, searchContent, tags)
}
