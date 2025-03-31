package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProxyHandler struct {
	store       *RequestStore
	certManager *CertManager
}

func NewProxyHandler(store *RequestStore, certManager *CertManager) *ProxyHandler {
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

	reqData := &RequestData{
		ID:        generateID(),
		Method:    r.Method,
		URL:       r.URL.String(),
		Headers:   r.Header,
		Body:      bodyBytes,
		Timestamp: time.Now(),
	}

	p.store.Save(reqData)

	return reqData, nil
}

func (p *ProxyHandler) saveResponse(id string, resp *http.Response) {
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	p.store.UpdateResponse(id, &ResponseData{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       bodyBytes,
	})
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
