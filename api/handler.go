package api

import (
	"bytes"
	"encoding/json"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"proxy-scanner/proxy"
	"strings"
)

type APIHandler struct {
	store   *proxy.RequestStore
	scanner *proxy.Scanner
}

func NewAPIHandler(store *proxy.RequestStore) *APIHandler {
	return &APIHandler{
		store:   store,
		scanner: proxy.NewScanner(store),
	}
}

func (h *APIHandler) listRequests(w http.ResponseWriter, r *http.Request) {
	requests := h.store.GetAll()
	json.NewEncoder(w).Encode(requests)
}

func (h *APIHandler) getRequest(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/requests/")
	req, exists := h.store.Get(id)
	if !exists {
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

	reqData, exists := h.store.Get(id)
	if !exists {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	req, err := http.NewRequest(reqData.Method, reqData.URL, bytes.NewReader(reqData.Body))
	if err != nil {
		http.Error(w, "Failed to recreate request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for k, values := range reqData.Headers {
		req.Header[k] = values
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Request failed: "+err.Error(), http.StatusBadGateway)
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
