package proxy

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

func (p *ProxyHandler) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = "443"
	}

	destConn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)
	if err != nil {
		log.Printf("Failed to connect to target: %v", err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	clientConn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Printf("Hijack failed: %v", err)
		return
	}

	go transfer(destConn, clientConn)
	transfer(clientConn, destConn)
}

func transfer(dst io.Writer, src io.Reader) {
	defer func() {
		if c, ok := dst.(io.Closer); ok {
			c.Close()
		}
		if c, ok := src.(io.Closer); ok {
			c.Close()
		}
	}()
	io.Copy(dst, src)
}
func (p *ProxyHandler) mitmConnection(clientConn net.Conn, destConn net.Conn, host string) {
	defer clientConn.Close()
	defer destConn.Close()

	cert, err := p.certManager.GetCert(host)
	if err != nil {
		log.Printf("Failed to generate cert for %s: %v", host, err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	go io.Copy(destConn, tlsConn)
	io.Copy(tlsConn, destConn)
}
