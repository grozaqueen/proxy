package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"proxy-scanner/proxy"
	"strings"
)

type Vulnerability struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

func (h *APIHandler) scanRequest(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID parameter is required", http.StatusBadRequest)
		return
	}

	reqData, exists := h.store.GetRequest(id)
	if exists != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	vulnerabilities := h.scanForVulnerabilities(reqData)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(vulnerabilities); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *APIHandler) scanForVulnerabilities(reqData *proxy.RequestData) []Vulnerability {
	var vulnerabilities []Vulnerability

	if _, ok := reqData.Headers["Proxy-Connection"]; ok {
		vulnerabilities = append(vulnerabilities, Vulnerability{
			Type:        "Insecure Header",
			Description: "Proxy-Connection header should be removed",
			Severity:    "Low",
		})
	}

	sqlKeywords := []string{"'", "\"", "--", ";", "UNION", "SELECT", "INSERT", "DELETE", "UPDATE"}
	for _, kw := range sqlKeywords {
		if strings.Contains(strings.ToUpper(reqData.URL), strings.ToUpper(kw)) {
			vulnerabilities = append(vulnerabilities, Vulnerability{
				Type:        "SQL Injection",
				Description: "Possible SQL injection in URL",
				Severity:    "High",
			})
			break
		}
	}

	for key, value := range reqData.BodyParams {
		if strings.Contains(strings.ToLower(value.Value), "<script>") {
			vulnerabilities = append(vulnerabilities, Vulnerability{
				Type:        "XSS",
				Description: fmt.Sprintf("Possible XSS in form parameter '%s'", key),
				Severity:    "Medium",
			})
		}

		for _, kw := range sqlKeywords {
			if strings.Contains(strings.ToUpper(value.Value), strings.ToUpper(kw)) {
				vulnerabilities = append(vulnerabilities, Vulnerability{
					Type:        "SQL Injection",
					Description: fmt.Sprintf("Possible SQL injection in form parameter '%s'", key),
					Severity:    "High",
				})
				break
			}
		}
	}

	if len(vulnerabilities) == 0 {
		vulnerabilities = append(vulnerabilities, Vulnerability{
			Type:        "No vulnerabilities found",
			Description: "No obvious vulnerabilities detected",
			Severity:    "None",
		})
	}

	return vulnerabilities
}

func checkForXXE(req *http.Request) error {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %v", err)
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(body))

	if strings.HasPrefix(string(body), "<?xml") {
		xxePayload := `<!DOCTYPE foo [
<!ELEMENT foo ANY >
<!ENTITY xxe SYSTEM "file:///etc/passwd" >]>
<foo>&xxe;</foo>`

		body = append([]byte(xxePayload), body...)
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
	}
	return nil
}

func checkXXEVulnerability(req *http.Request) (bool, error) {
	client := &http.Client{}

	if err := checkForXXE(req); err != nil {
		return false, fmt.Errorf("failed to modify request for XXE: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	if strings.Contains(string(body), "root:") {
		return true, nil
	}

	return false, nil
}

func (h *APIHandler) checkRequestForXXE(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "ID parameter is required", http.StatusBadRequest)
		return
	}

	req, err := h.store.GetRequest(id)
	if err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	httpReq, err := http.NewRequest(req.Parsed.Method, req.Parsed.Scheme+"://"+req.Parsed.Host+req.Parsed.Path, bytes.NewReader(req.Parsed.RawBody))
	if err != nil {
		http.Error(w, "Failed to create HTTP request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	isVulnerable, err := checkXXEVulnerability(httpReq)
	if err != nil {
		http.Error(w, "Error checking for XXE vulnerability: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isVulnerable {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request is vulnerable to XXE"))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Request is not vulnerable to XXE"))
	}
}
