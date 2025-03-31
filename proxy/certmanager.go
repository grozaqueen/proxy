package proxy

import (
	"crypto/tls"
	"log"
	"os/exec"
	"path/filepath"
	"sync"
)

type CertManager struct {
	cacheDir string
	caCert   tls.Certificate
	mu       sync.Mutex
	certs    map[string]tls.Certificate
}

func NewCertManager(cacheDir string) (*CertManager, error) {
	caCert, err := tls.LoadX509KeyPair("certs/ca.crt", "certs/ca.key")
	if err != nil {
		return nil, err
	}

	return &CertManager{
		cacheDir: cacheDir,
		caCert:   caCert,
		certs:    make(map[string]tls.Certificate),
	}, nil
}

func (cm *CertManager) GenerateCert(host string) (tls.Certificate, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	log.Printf("Generating certificate for: %s", host)

	if cert, ok := cm.certs[host]; ok {
		log.Printf("Using cached certificate for: %s", host)
		return cert, nil
	}

	cmd := exec.Command("./gen_cert.sh", host)
	cmd.Dir = cm.cacheDir
	log.Printf("Executing: %v", cmd.Args)

	if err := cmd.Run(); err != nil {
		log.Printf("Command failed: %v", err)
		return tls.Certificate{}, err
	}

	certFile := filepath.Join(cm.cacheDir, host+".crt")
	keyFile := filepath.Join(cm.cacheDir, host+".key")
	log.Printf("Loading cert: %s, key: %s", certFile, keyFile)

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Printf("LoadX509KeyPair error: %v", err)
		return tls.Certificate{}, err
	}

	cm.certs[host] = cert
	return cert, nil
}

func (cm *CertManager) GetCert(host string) (tls.Certificate, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cert, ok := cm.certs[host]; ok {
		return cert, nil
	}

	cmd := exec.Command("./certs/gen_cert.sh", host)
	cmd.Dir = cm.cacheDir
	if err := cmd.Run(); err != nil {
		return tls.Certificate{}, err
	}

	cert, err := tls.LoadX509KeyPair(
		filepath.Join(cm.cacheDir, host+".crt"),
		filepath.Join(cm.cacheDir, host+".key"),
	)
	if err != nil {
		return tls.Certificate{}, err
	}

	cm.certs[host] = cert
	return cert, nil
}
