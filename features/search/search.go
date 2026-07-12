package search

import (
	"arkana/config"
	"arkana/features/search/handlers"
	"arkana/features/search/services"
	"database/sql"

	"github.com/gorilla/mux"
)

func Initialize(router *mux.Router, db *sql.DB, cfg *config.Config) {
	service := services.NewSearchService(db, cfg.MeiliHost, cfg.MeiliMasterKey)

	handlers.RegisterRoutes(router, service)
}
