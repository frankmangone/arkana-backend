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

// WriterResolver resolves a frontmatter author slug to a writer's internal
// id, for linking published posts to their writer so post_liked/
// post_commented notifications can be routed. Satisfied structurally by
// *writers/services.WriterService, without this package depending on it
// directly.
type WriterResolver interface {
	GetIDBySlug(slug string) (id int64, found bool, err error)
}

var ErrUnknownTags = errors.New("unknown tag(s)")
var ErrMissingAuthor = errors.New("post is missing required frontmatter field: author")
var ErrUnknownAuthor = errors.New("unknown author")

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
	writers WriterResolver
}

// NewAdminPostService constructs an AdminPostService backed by db, composing
// posts for posts-row access, indexer for search indexing, tags for tag
// validation, and writers for resolving a post's author to a writer.
func NewAdminPostService(db *sql.DB, posts *PostService, indexer PostIndexer, tags TagChecker, writers WriterResolver) *AdminPostService {
	return &AdminPostService{db: db, queries: queries.NewSQLAdminPostQueries(db), posts: posts, indexer: indexer, tags: tags, writers: writers}
}

// Publish parses RawContent's frontmatter for
// title/thumbnail/description/tags/author, resolves author to a writer and
// links it via posts.writer_id (so post_liked/post_commented notifications
// can be routed), upserts the posts/post_contents rows for one (path, lang)
// with the full raw content (frontmatter included, since pull-content.js
// writes this column's value out verbatim as the served .md file), and
// indexes a search-stripped version of the body into search. Re-publishing
// the same path/lang updates the existing row rather than duplicating it.
func (s *AdminPostService) Publish(input PublishInput) error {
	frontmatter, body, err := parseFrontmatter(input.RawContent)
	if err != nil {
		return fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	title := frontmatterString(frontmatter, "title")
	thumbnail := frontmatterString(frontmatter, "thumbnail")
	description := frontmatterString(frontmatter, "description")
	tags := frontmatterStringSlice(frontmatter, "tags")
	author := frontmatterString(frontmatter, "author")

	if len(tags) > 0 {
		if missing, err := s.tags.MissingTags(tags); err != nil {
			return err
		} else if len(missing) > 0 {
			return fmt.Errorf("%w: %s", ErrUnknownTags, strings.Join(missing, ", "))
		}
	}

	if author == "" {
		return fmt.Errorf("%w (path %q)", ErrMissingAuthor, input.Path)
	}
	writerID, found, err := s.writers.GetIDBySlug(author)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%w: %q (path %q)", ErrUnknownAuthor, author, input.Path)
	}

	post, err := s.posts.GetOrCreateByPath(input.Path)
	if err != nil {
		return err
	}

	if err := s.posts.SetWriterID(post.ID, writerID); err != nil {
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
