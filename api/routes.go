package api

import (
	"github.com/gorilla/mux"
)

func NewRouter(handler *APIHandler) *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/requests", handler.listRequests).Methods("GET")
	r.HandleFunc("/requests/{id}", handler.getRequest).Methods("GET")
	r.HandleFunc("/repeat/{id}", handler.repeatRequest).Methods("GET")
	r.HandleFunc("/scan/{id}", handler.repeatRequest).Methods("GET")

	return r
}
