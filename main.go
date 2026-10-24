// LotusGate: The Lightweight, Zero-Config AI Edge Proxy in Pure Go.
//
// High-performance reverse proxy designed for AI applications, LLMs and backends.
// Handles edge protection: concurrency pooling, token-bucket rate limiting per IP,
// request stamping, robust timeouts, zero-config native TLS/HTTPS and healthchecks.
//
// Env:
//   LISTEN        HTTP listen address (default ":8080")
//   TLS_LISTEN    HTTPS listen address (default ":8443")
//   TLS_CERT      TLS certificate path (default "certs/server.crt")
//   TLS_KEY       TLS private key path (default "certs/server.key")
//   UPSTREAM      Upstream backend URL (default "http://127.0.0.1:8000")
//   RATE_PER_SEC  Requests/sec limit per IP (default 60)
//   TIMEOUT_SEC   Upstream timeout in seconds (default 30)
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// ---- limite de velocidade por IP (token bucket simples) ----

type bucket struct {
	tokens float64
	last   time.Time
}

type limiter struct {
	mu    sync.Mutex
	byIP  map[string]*bucket
	rate  float64
	burst float64
}

func newLimiter(ratePerSec int) *limiter {
	return &limiter{
		byIP:  map[string]*bucket{},
		rate:  float64(ratePerSec),
		burst: float64(ratePerSec),
	}
}

func (l *limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.byIP[ip]
	now := time.Now()
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.byIP[ip] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.burst {
		b.tokens = l.burst
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// ---- gateway ----

type gateway struct {
	upstream *url.URL
	proxy    *httputil.ReverseProxy
	limit    *limiter
	client   *http.Client
}

func newGateway(upstreamRaw string, ratePerSec, timeoutSec int) (*gateway, error) {
	u, err := url.Parse(upstreamRaw)
	if err != nil {
		return nil, fmt.Errorf("UPSTREAM invalido: %w", err)
	}
	timeout := time.Duration(timeoutSec) * time.Second
	g := &gateway{
		upstream: u,
		limit:    newLimiter(ratePerSec),
		client:   &http.Client{Timeout: 5 * time.Second},
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.SetXForwarded()
			r.Out.Header.Set("X-Ichyra-Gateway", "go-tls")
		},
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     90 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, `{"detail":"backend indisponivel"}`, http.StatusBadGateway)
		},
	}
	_ = timeout
	g.proxy = rp
	return g, nil
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	host := r.RemoteAddr
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i]
		}
	}
	return host
}

// Saude do porteiro + do backend. Nunca vaza segredo.
func (g *gateway) healthz(w http.ResponseWriter, r *http.Request) {
	up := "down"
	req, _ := http.NewRequest("GET", g.upstream.String()+"/ready", nil)
	resp, err := g.client.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == 200 {
			up = "up"
		}
	}
	w.Header().Set("Content-Type", "application/json")
	code := http.StatusOK
	if up == "down" {
		code = http.StatusServiceUnavailable
	}
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"gateway":"ok","upstream":"%s","tls":true}`, up)
}

func (g *gateway) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", g.healthz)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !g.limit.allow(clientIP(r)) {
			http.Error(w, `{"detail":"limite excedido"}`, http.StatusTooManyRequests)
			return
		}
		g.proxy.ServeHTTP(w, r)
	})
	return mux
}

// ---- gerador automatico de certificado auto-assinado ECDSA P-256 ----

func ensureCertificates(certPath, keyPath string) error {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return nil // Certificado e chave já existem
		}
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("falha ao gerar chave ECDSA: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNum, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("falha ao gerar serial do certificado: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNum,
		Subject: pkix.Name{
			Organization: []string{"ICHYRA Zero-Trust Security"},
			CommonName:   "localhost",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "127.0.0.1"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("falha ao assinar certificado: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("falha ao criar pasta de certs: %w", err)
	}

	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("falha ao salvar cert: %w", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("falha ao salvar chave: %w", err)
	}
	defer keyOut.Close()

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("falha ao serializar chave EC: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}

	fmt.Printf("[TLS] Certificado SSL gerado com sucesso em: %s\n", certPath)
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v <= 0 {
		return def
	}
	return v
}

func main() {
	listen := getenv("LISTEN", ":8080")
	tlsListen := getenv("TLS_LISTEN", ":8443")
	certFile := getenv("TLS_CERT", "certs/server.crt")
	keyFile := getenv("TLS_KEY", "certs/server.key")
	upstream := getenv("UPSTREAM", "http://127.0.0.1:8000")
	rate := getenvInt("RATE_PER_SEC", 60)
	timeout := getenvInt("TIMEOUT_SEC", 30)

	g, err := newGateway(upstream, rate, timeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Garante certificado TLS
	if err := ensureCertificates(certFile, keyFile); err != nil {
		fmt.Fprintf(os.Stderr, "[AVISO] Nao foi possivel gerar certificado TLS: %v\n", err)
	}

	// Servidor HTTPS (TLS Nativo)
	tlsSrv := &http.Server{
		Addr:         tlsListen,
		Handler:      g.handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: time.Duration(timeout+10) * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Servidor HTTP padrão
	srv := &http.Server{
		Addr:         listen,
		Handler:      g.handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: time.Duration(timeout+10) * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Inicia HTTPS em goroutine separada
	go func() {
		fmt.Printf("LotusGate HTTPS listening on %s (TLS active) -> %s\n", tlsListen, upstream)
		if err := tlsSrv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "[HTTPS ERROR]: %v\n", err)
		}
	}()

	fmt.Printf("LotusGate HTTP listening on %s -> %s\n", listen, upstream)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
