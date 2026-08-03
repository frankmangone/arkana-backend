package quizzes

import (
	"database/sql"

	"arkana/features/auth/middlewares"
	postsservices "arkana/features/posts/services"
	"arkana/features/quizzes/handlers"
	"arkana/features/quizzes/services"
	tagsservices "arkana/features/tags/services"
	"arkana/shared/adminauth"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
)

// Initialize constructs its own *postsservices.PostService and
// *tagsservices.TagService instances against the shared db, same pattern
// features/readinglists uses for its own PostService instance.
func Initialize(router *mux.Router, db *sql.DB, redisClient *redis.Client, adminAuth *adminauth.AdminAuthMiddleware, auth *middlewares.AuthMiddleware) {
	postService := postsservices.NewPostService(db, nil)
	tagService := tagsservices.NewTagService(db)
	questionService := services.NewQuestionService(db, postService, tagService)
	sessionService := services.NewQuizSessionService(db, redisClient)
	handlers.RegisterRoutes(router, questionService, sessionService, auth, adminAuth)
}
