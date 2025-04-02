// api/handlers.go
package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
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

	reqData, exists := h.store.Get(id)
	if !exists {
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
