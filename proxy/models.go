package proxy

import (
	"net/http"
	"sync"
	"time"
)

type Param struct {
	Key   string
	Value string
}

type RequestData struct {
	ID         string
	Method     string
	URL        string
	Headers    http.Header
	Body       []byte
	BodyParams []Param
	Timestamp  time.Time
	Response   *ResponseData
	Parsed     ParsedRequest
}

type ResponseData struct {
	Status     string
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type RequestStore struct {
	requests map[string]*RequestData
	mu       sync.RWMutex
}

func NewRequestStore() *RequestStore {
	return &RequestStore{
		requests: make(map[string]*RequestData),
	}
}

func (s *RequestStore) Save(req *RequestData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests[req.ID] = req
}

func (s *RequestStore) Get(id string) (*RequestData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, exists := s.requests[id]
	return req, exists
}

func (s *RequestStore) GetAll() []*RequestData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*RequestData
	for _, req := range s.requests {
		result = append(result, req)
	}
	return result
}

func (s *RequestStore) UpdateResponse(id string, resp *ResponseData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req, exists := s.requests[id]; exists {
		req.Response = resp
	}
}

type ParsedRequest struct {
	Method     string `json:"method"`
	Scheme     string
	Host       string
	Path       string            `json:"path"`
	GetParams  map[string]string `json:"get_params"`
	Headers    map[string]string `json:"headers"`
	Cookies    map[string]string `json:"cookies"`
	PostParams map[string]string `json:"post_params"`
	RawBody    []byte            `json:"raw_body"`
}

type ParsedResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body"`
}

type RequestRecord struct {
	ID        string
	Request   ParsedRequest
	Response  ParsedResponse
	Timestamp time.Time
}
