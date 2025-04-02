package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CertManager struct {
	mu      sync.Mutex
	caCert  tls.Certificate
	certDir string
	certs   map[string]tls.Certificate // Единое поле для кэша (было certCache и certs)
}

func NewCertManager(certDir string) (*CertManager, error) {
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate directory does not exist: %s", certDir)
	}

	caCert, err := tls.LoadX509KeyPair(
		filepath.Join(certDir, "ca.crt"),
		filepath.Join(certDir, "ca.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %v", err)
	}

	return &CertManager{
		caCert:  caCert,
		certDir: certDir,
		certs:   make(map[string]tls.Certificate), // Инициализация кэша
	}, nil
}

func (cm *CertManager) GenerateCert(host string) (tls.Certificate, error) {
	normalizedHost := strings.Split(host, ":")[0]

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cert, ok := cm.certs[normalizedHost]; ok {
		log.Printf("Using cached certificate for: %s", normalizedHost)
		return cert, nil
	}

	log.Printf("Generating new certificate for: %s", normalizedHost)

	cmd := exec.Command("/bin/bash", "/app/certs/gen_cert.sh", normalizedHost)
	cmd.Dir = "/app/certs"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return tls.Certificate{}, fmt.Errorf("certificate generation failed: %v", err)
	}

	certPath := filepath.Join(cm.certDir, normalizedHost)
	cert, err := tls.LoadX509KeyPair(
		certPath+".crt",
		certPath+".key",
	)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to load certificate: %v", err)
	}

	if err := cm.validateCertificate(cert); err != nil {
		return tls.Certificate{}, err
	}

	cm.certs[normalizedHost] = cert
	return cert, nil
}

func (cm *CertManager) GetCert(host string) (tls.Certificate, error) {
	return cm.GenerateCert(host)
}

func (cm *CertManager) validateCertificate(cert tls.Certificate) error {
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("certificate parsing failed: %v", err)
	}

	if time.Now().After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate expired")
	}

	if x509Cert.Subject.CommonName == "" {
		return fmt.Errorf("certificate has empty Common Name")
	}

	return nil
}
