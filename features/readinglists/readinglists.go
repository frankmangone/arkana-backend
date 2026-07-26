package readinglists

import (
	"database/sql"

	"arkana/features/readinglists/handlers"
	"arkana/features/readinglists/services"
	postsservices "arkana/features/posts/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
)

// Initialize constructs its own *postsservices.PostService instance
// against the shared db, same pattern features/posts uses for its own
// *tagservices.TagService instance - each feature builds the dependency
// it needs directly, no cross-feature parameter threading through
// router.go. nil for the notifications parameter is safe here:
// MissingPaths only ever touches the posts table, never
// s.notifications - that field is only dereferenced by
// ToggleLike/ToggleRead's notify-on-like path, neither of which this
// module calls.
func Initialize(router *mux.Router, db *sql.DB, adminAuth *adminauth.AdminAuthMiddleware) {
	postService := postsservices.NewPostService(db, nil)
	svc := services.NewReadingListService(db, postService)
	handlers.RegisterRoutes(router, svc, adminAuth)
}
