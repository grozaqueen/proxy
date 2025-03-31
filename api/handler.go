package api

import (
	"bytes"
	"encoding/json"
	"io"
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
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	reqData, exists := h.store.Get(id)
	if !exists {
		http.NotFound(w, r)
		return
	}

	req, err := http.NewRequest(reqData.Method, reqData.URL, bytes.NewReader(reqData.Body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for k, vv := range reqData.Headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (h *APIHandler) scanRequest(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	vulnerabilities, err := h.scanner.ScanRequest(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(vulnerabilities)
}
