package handlers

import (
	"arkana/features/search/services"

	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, service *services.SearchService) {
	searchHandler := NewSearchHandler(service)

	router.HandleFunc("/api/search", searchHandler.Search).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/search/tags", searchHandler.SearchTags).Methods("GET", "OPTIONS")
}
