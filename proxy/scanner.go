package proxy

import (
	"fmt"
	"strings"
)

type Scanner struct {
	store *RequestStore
}

func NewScanner(store *RequestStore) *Scanner {
	return &Scanner{store: store}
}

func (s *Scanner) ScanRequest(id string) ([]string, error) {
	reqData, exists := s.store.Get(id)
	if !exists {
		return nil, fmt.Errorf("request not found")
	}

	var vulnerabilities []string

	if reqData.Response != nil {
		if _, ok := reqData.Response.Headers["X-Powered-By"]; ok {
			vulnerabilities = append(vulnerabilities, "Information disclosure: X-Powered-By header exposed")
		}

		if _, ok := reqData.Response.Headers["X-XSS-Protection"]; !ok {
			vulnerabilities = append(vulnerabilities, "Missing security header: X-XSS-Protection")
		}

		if _, ok := reqData.Response.Headers["Content-Security-Policy"]; !ok {
			vulnerabilities = append(vulnerabilities, "Missing security header: Content-Security-Policy")
		}

		if strings.Contains(reqData.URL, "' OR '1'='1") || strings.Contains(reqData.URL, "1=1") {
			vulnerabilities = append(vulnerabilities, "Possible SQL injection in URL parameters")
		}

		if strings.Contains(reqData.URL, "<script>") || strings.Contains(reqData.URL, "alert(") {
			vulnerabilities = append(vulnerabilities, "Possible XSS in URL parameters")
		}
	}

	return vulnerabilities, nil
}
