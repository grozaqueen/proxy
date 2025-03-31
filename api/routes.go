package api

import (
	"github.com/gorilla/mux"
)

func NewRouter(handler *APIHandler) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/requests", handler.listRequests).Methods("GET")
	r.HandleFunc("/requests/{id}", handler.getRequest).Methods("GET")
	r.HandleFunc("/repeat", handler.repeatRequest).Methods("GET").Queries("id", "{id}")
	r.HandleFunc("/scan", handler.scanRequest).Methods("GET").Queries("id", "{id}")

	return r
}
