package proxy

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProxyHandler struct {
	store       *DBStore
	certManager *CertManager
}

func NewProxyHandler(store *DBStore, certManager *CertManager) *ProxyHandler {
	return &ProxyHandler{
		store:       store,
		certManager: certManager,
	}
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleHTTPS(w, r)
		return
	}
	reqData, err := p.saveRequest(r)
	if err != nil {
		http.Error(w, "Error saving request", http.StatusInternalServerError)
		return
	}

	modifiedReq, err := p.modifyRequest(r)
	if err != nil {
		http.Error(w, "Error modifying request", http.StatusBadRequest)
		return
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(modifiedReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error forwarding request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	p.saveResponse(reqData.ID, resp)

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func (p *ProxyHandler) modifyRequest(r *http.Request) (*http.Request, error) {
	originalURL := r.URL.String()
	if !strings.HasPrefix(originalURL, "http") {
		originalURL = "http://" + originalURL
	}

	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return nil, err
	}

	newReq, err := http.NewRequest(r.Method, parsedURL.String(), r.Body)
	if err != nil {
		return nil, err
	}

	for k, vv := range r.Header {
		if strings.ToLower(k) == "proxy-connection" {
			continue
		}
		for _, v := range vv {
			newReq.Header.Add(k, v)
		}
	}

	newReq.ContentLength = r.ContentLength
	newReq.TransferEncoding = r.TransferEncoding
	newReq.Close = r.Close
	newReq.Host = r.Host
	fmt.Printf("Modified request: %s %s HTTP/1.1\n", newReq.Method, newReq.URL.Path)
	return newReq, nil
}

func (p *ProxyHandler) saveRequest(r *http.Request) (*RequestData, error) {
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	parsedReq := ParsedRequest{
		Method:     r.Method,
		Scheme:     r.URL.Scheme,
		Host:       r.Host,
		Path:       r.URL.Path,
		GetParams:  make(map[string]string),
		Headers:    make(map[string]string),
		Cookies:    make(map[string]string),
		PostParams: make(map[string]string),
		RawBody:    bodyBytes,
	}

	if parsedReq.Scheme == "" {
		if r.TLS != nil {
			parsedReq.Scheme = "https"
		} else {
			parsedReq.Scheme = "http"
		}
	}

	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			parsedReq.GetParams[k] = v[0]
		}
	}

	for k, v := range r.Header {
		parsedReq.Headers[k] = strings.Join(v, ", ")
		if k == "Cookie" {
			cookies := strings.Split(v[0], ";")
			for _, cookie := range cookies {
				parts := strings.SplitN(strings.TrimSpace(cookie), "=", 2)
				if len(parts) == 2 {
					parsedReq.Cookies[parts[0]] = parts[1]
				}
			}
		}
	}

	if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
		if err := r.ParseForm(); err == nil {
			for k, v := range r.PostForm {
				if len(v) > 0 {
					parsedReq.PostParams[k] = v[0]
				}
			}
		}
	}

	reqData := &RequestData{
		ID:        generateID(),
		Method:    r.Method,
		URL:       r.URL.String(),
		Headers:   r.Header,
		Body:      bodyBytes,
		Timestamp: time.Now(),
		Parsed:    parsedReq,
	}

	log.Printf("Request received: %v", r)

	err := p.store.SaveRequest(reqData)
	if err != nil {
		log.Printf("Error saving request: %v", err)
		return nil, err
	}

	return reqData, nil
}

func (p *ProxyHandler) saveResponse(id string, resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gr, err := gzip.NewReader(bytes.NewReader(bodyBytes))
		if err == nil {
			defer gr.Close()
			decompressed, err := io.ReadAll(gr)
			if err == nil {
				bodyBytes = decompressed
			}
		}
	}

	parsedResp := ParsedResponse{
		Code:    resp.StatusCode,
		Message: resp.Status,
		Headers: make(map[string]string),
		Body:    bodyBytes,
	}

	for k, v := range resp.Header {
		parsedResp.Headers[k] = strings.Join(v, ", ")
	}

	return p.store.UpdateResponse(id, parsedResp)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

type DBStore struct {
	db *sql.DB
}

func NewDBStore(dsn string) (*DBStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to DB: %w", err)
	}

	log.Println("[DB] Successfully connected")
	return &DBStore{db: db}, nil
}

func (s *DBStore) SaveRequest(req *RequestData) error {
	getParams, err := toJSONB(req.Parsed.GetParams)
	if err != nil {
		return err
	}

	headers, err := toJSONB(req.Parsed.Headers)
	if err != nil {
		return err
	}

	cookies, err := toJSONB(req.Parsed.Cookies)
	if err != nil {
		return err
	}

	postParams, err := toJSONB(req.Parsed.PostParams)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
    INSERT INTO requests (
        id, method, scheme, host, path, get_params, headers, cookies, 
        post_params, raw_body, timestamp
    ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`,
		req.ID,
		req.Parsed.Method,
		req.Parsed.Scheme,
		req.Parsed.Host,
		req.Parsed.Path,
		getParams,
		headers,
		cookies,
		postParams,
		req.Parsed.RawBody,
		req.Timestamp,
	)

	return err
}

func (s *DBStore) UpdateResponse(id string, resp ParsedResponse) error {
	headers, err := toJSONB(resp.Headers)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
        UPDATE requests SET
            response_code = $1,
            response_message = $2,
            response_headers = $3,
            response_body = $4
        WHERE id = $5
    `,
		resp.Code,
		resp.Message,
		headers,
		resp.Body,
		id,
	)
	return err
}

func toJSONB(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

func (s *DBStore) GetRequest(id string) (*RequestData, error) {
	var req RequestData
	var getParams, headers, cookies, postParams, rawBody []byte
	var responseHeaders []byte
	var timestamp time.Time

	req.Response = &ResponseData{}

	err := s.db.QueryRow(`
        SELECT 
            id, method, scheme, host, path, get_params, headers, cookies, 
            post_params, raw_body, timestamp,
            response_code, response_message, response_headers, response_body
        FROM requests WHERE id = $1
    `, id).Scan(
		&req.ID,
		&req.Parsed.Method,
		&req.Parsed.Scheme,
		&req.Parsed.Host,
		&req.Parsed.Path,
		&getParams,
		&headers,
		&cookies,
		&postParams,
		&rawBody,
		&timestamp,
		&req.Response.StatusCode,
		&req.Response.Status,
		&responseHeaders,
		&req.Response.Body,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		log.Printf("Error getting request from DB: %v", err)
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(responseHeaders, &raw); err != nil {
		log.Printf("[DB] Failed to unmarshal response_headers: %v\n", err)
		return nil, err
	}
	req.Response.Headers = make(http.Header)
	for k, v := range raw {
		switch vv := v.(type) {
		case string:
			req.Response.Headers[k] = []string{vv}
		case []interface{}:
			for _, item := range vv {
				if str, ok := item.(string); ok {
					req.Response.Headers[k] = append(req.Response.Headers[k], str)
				}
			}
		}
	}

	json.Unmarshal(getParams, &req.Parsed.GetParams)
	json.Unmarshal(headers, &req.Parsed.Headers)
	json.Unmarshal(cookies, &req.Parsed.Cookies)
	json.Unmarshal(postParams, &req.Parsed.PostParams)
	req.Parsed.RawBody = rawBody
	req.Timestamp = timestamp

	return &req, nil
}

func (p *ProxyHandler) ResendRequest(id string) (*http.Response, error) {
	reqRecord, err := p.store.GetRequest(id)
	if err != nil {
		return nil, err
	}

	if reqRecord.Parsed.Scheme == "" || reqRecord.Parsed.Host == "" {
		return nil, fmt.Errorf("invalid request: missing scheme or host")
	}

	urlStr := reqRecord.Parsed.Scheme + "://" + reqRecord.Parsed.Host + reqRecord.Parsed.Path
	if len(reqRecord.Parsed.GetParams) > 0 {
		q := url.Values{}
		for k, v := range reqRecord.Parsed.GetParams {
			q.Add(k, v)
		}
		urlStr += "?" + q.Encode()
	}

	var body io.Reader
	if len(reqRecord.Parsed.RawBody) > 0 {
		body = bytes.NewReader(reqRecord.Parsed.RawBody)
	}

	req, err := http.NewRequest(reqRecord.Parsed.Method, urlStr, body)
	if err != nil {
		return nil, err
	}

	for k, v := range reqRecord.Parsed.Headers {
		req.Header.Set(k, v)
	}

	for k, v := range reqRecord.Parsed.Cookies {
		req.AddCookie(&http.Cookie{Name: k, Value: v})
	}

	client := &http.Client{}
	return client.Do(req)
}

func (s *DBStore) GetAll() ([]*RequestData, error) {
	log.Println("[DB] Executing query to get all requests")

	rows, err := s.db.Query(`
        SELECT 
            id, method, scheme, host, path, get_params, headers, cookies, 
            post_params, raw_body, timestamp,
            response_code, response_message, response_headers, response_body
        FROM requests
        ORDER BY timestamp DESC
    `)
	if err != nil {
		log.Printf("[DB] Failed to query requests: %v\n", err)
		return nil, fmt.Errorf("failed to query requests: %v", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			log.Printf("[DB] Failed to close rows: %v\n", cerr)
		}
	}()

	var requests []*RequestData

	for rows.Next() {
		var req RequestData
		var getParams, headers, cookies, postParams []byte
		var responseHeaders []byte

		req.Parsed = ParsedRequest{
			GetParams:  make(map[string]string),
			Headers:    make(map[string]string),
			Cookies:    make(map[string]string),
			PostParams: make(map[string]string),
		}

		req.Response = &ResponseData{}

		var responseCode sql.NullInt64

		err := rows.Scan(
			&req.ID,
			&req.Parsed.Method,
			&req.Parsed.Scheme,
			&req.Parsed.Host,
			&req.Parsed.Path,
			&getParams,
			&headers,
			&cookies,
			&postParams,
			&req.Parsed.RawBody,
			&req.Timestamp,
			&responseCode,
			&req.Response.Status,
			&responseHeaders,
			&req.Response.Body,
		)
		if err != nil {
			log.Printf("[DB] Failed to scan row: %v\n", err)
			return nil, fmt.Errorf("failed to scan request row: %v", err)
		}

		if responseCode.Valid {
			req.Response.StatusCode = int(responseCode.Int64)
		} else {
			req.Response.StatusCode = 0
		}

		if err != nil {
			log.Printf("[DB] Failed to scan row: %v\n", err)
			return nil, fmt.Errorf("failed to scan request row: %v", err)
		}

		if len(getParams) > 0 {
			_ = json.Unmarshal(getParams, &req.Parsed.GetParams)
		}
		if len(headers) > 0 {
			_ = json.Unmarshal(headers, &req.Parsed.Headers)
		}
		if len(cookies) > 0 {
			_ = json.Unmarshal(cookies, &req.Parsed.Cookies)
		}
		if len(postParams) > 0 {
			_ = json.Unmarshal(postParams, &req.Parsed.PostParams)
		}
		if len(responseHeaders) > 0 {
			var rawHeaders map[string]interface{}
			if err := json.Unmarshal(responseHeaders, &rawHeaders); err == nil {
				req.Response.Headers = make(http.Header)
				for k, v := range rawHeaders {
					switch vv := v.(type) {
					case string:
						req.Response.Headers[k] = []string{vv}
					case []interface{}:
						for _, item := range vv {
							if str, ok := item.(string); ok {
								req.Response.Headers[k] = append(req.Response.Headers[k], str)
							}
						}
					}
				}
			}
		}

		requests = append(requests, &req)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[DB] Rows iteration error: %v\n", err)
		return nil, fmt.Errorf("rows iteration error: %v", err)
	}

	log.Printf("[DB] Successfully loaded %d requests\n", len(requests))
	return requests, nil
}
