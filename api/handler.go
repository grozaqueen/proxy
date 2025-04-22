package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"proxy-scanner/proxy"
	"strings"
)

type APIHandler struct {
	store        *proxy.DBStore
	scanner      *proxy.Scanner
	proxyHandler *proxy.ProxyHandler
}

func NewAPIHandler(store *proxy.DBStore, proxyHandler *proxy.ProxyHandler) *APIHandler {
	fmt.Printf("[API] NewAPIHandler created with store: %+v", store)
	return &APIHandler{
		store:        store,
		scanner:      proxy.NewScanner(store),
		proxyHandler: proxyHandler,
	}
}

func (h *APIHandler) listRequests(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		fmt.Println("[ERROR] h.store is nil in listRequests")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	requests, _ := h.store.GetAll()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(requests)
}

func (h *APIHandler) getRequest(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/requests/")
	req, err := h.store.GetRequest(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	json.NewEncoder(w).Encode(req)
}

func (h *APIHandler) repeatRequest(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID parameter is required", http.StatusBadRequest)
		return
	}

	resp, err := h.proxyHandler.ResendRequest(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Request not found", http.StatusNotFound)
		} else {
			http.Error(w, "Request failed: "+err.Error(), http.StatusBadGateway)
		}
		return
	}
	defer resp.Body.Close()

	for k, values := range resp.Header {
		w.Header()[k] = values
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Warning: error while copying response body: %v", err)
	}
}
