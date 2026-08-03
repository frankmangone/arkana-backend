package services

import (
	"arkana/features/posts/queries"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// PostIndexer indexes a post into the search backend. Satisfied
// structurally by *search/services.SearchService, without this package
// depending on it directly.
type PostIndexer interface {
	IndexPost(lang, path, title, description, content string, tags []string) error
}

// TagChecker validates that a set of tag slugs are all registered tags.
// Satisfied structurally by *tags/services.TagService, without this
// package depending on it directly.
type TagChecker interface {
	MissingTags(slugs []string) ([]string, error)
}

var ErrUnknownTags = errors.New("unknown tag(s)")

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
	queries queries.AdminPostQueries
	posts   *PostService
	indexer PostIndexer
	tags    TagChecker
}

// NewAdminPostService constructs an AdminPostService backed by db, composing
// posts for posts-row access, indexer for search indexing, and tags for tag
// validation.
func NewAdminPostService(db *sql.DB, posts *PostService, indexer PostIndexer, tags TagChecker) *AdminPostService {
	return &AdminPostService{db: db, queries: queries.NewSQLAdminPostQueries(db), posts: posts, indexer: indexer, tags: tags}
}

// Publish parses RawContent's frontmatter for title/thumbnail/description/tags,
// upserts the posts/post_contents rows for one (path, lang) with the full
// raw content (frontmatter included, since pull-content.js writes this
// column's value out verbatim as the served .md file), and indexes a
// search-stripped version of the body into search. Re-publishing the same
// path/lang updates the existing row rather than duplicating it.
func (s *AdminPostService) Publish(input PublishInput) error {
	frontmatter, body, err := parseFrontmatter(input.RawContent)
	if err != nil {
		return fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	title := frontmatterString(frontmatter, "title")
	thumbnail := frontmatterString(frontmatter, "thumbnail")
	description := frontmatterString(frontmatter, "description")
	tags := frontmatterStringSlice(frontmatter, "tags")

	if len(tags) > 0 {
		if missing, err := s.tags.MissingTags(tags); err != nil {
			return err
		} else if len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrUnknownTags, strings.Join(missing, ", "))
		}
	}

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

	if err := s.queries.UpsertPostContent(post.ID, input.Lang, contentPath, input.RawContent, titleCol, thumbnailCol); err != nil {
		return err
	}

	searchContent := stripMarkdownForSearch(body)

	return s.indexer.IndexPost(input.Lang, input.Path, title, description, searchContent, tags)
}
