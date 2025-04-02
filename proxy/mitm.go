package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

func (p *ProxyHandler) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer clientConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		log.Printf("Failed to send 200 OK: %v", err)
		return
	}

	cert, err := p.certManager.GenerateCert(host)
	if err != nil {
		log.Printf("Certificate generation failed for %s: %v", host, err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   host,
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake with client failed: %v", err)
		return
	}

	destConn, err := tls.Dial("tcp", r.Host, &tls.Config{
		ServerName: host,
	})
	if err != nil {
		log.Printf("Failed to connect to target %s: %v", r.Host, err)
		return
	}
	defer destConn.Close()

	go func() {
		io.Copy(destConn, tlsConn)
		destConn.Close()
	}()

	io.Copy(tlsConn, destConn)
}

func (p *ProxyHandler) mitmConnection(clientConn net.Conn, host string) {
	defer clientConn.Close()

	cert, err := p.certManager.GenerateCert(host)
	if err != nil {
		log.Printf("Failed to generate cert for %s: %v", host, err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   host,
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake failed for %s: %v", host, err)
		return
	}

	reader := bufio.NewReader(tlsConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Printf("Failed to read request from %s: %v", host, err)
		return
	}
	defer req.Body.Close()

	req.URL.Scheme = "https"
	req.URL.Host = host
	req.RequestURI = ""

	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authenticate")
	req.Header.Del("Proxy-Authorization")

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		log.Printf("Failed to make request to %s: %v", host, err)
		return
	}
	defer resp.Body.Close()

	if err := resp.Write(tlsConn); err != nil {
		log.Printf("Failed to write response to client: %v", err)
	}
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(p)
	}
}

func isClosedConnError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection") ||
		strings.Contains(err.Error(), "connection reset by peer")
}
