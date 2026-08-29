package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type response struct {
	Component string `json:"component"`
	Path      string `json:"path"`
	Status    string `json:"status"`
}

func main() {
	component := getenv("A6_TARGET_NAME", "a6-target")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(response{
			Component: component,
			Path:      r.URL.Path,
			Status:    "reached",
		})
	})

	plain := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	mtlsConfig, err := mtlsServerConfig()
	if err != nil {
		slog.Error("failed to create mTLS config", "error", err)
		os.Exit(1)
	}
	mtls := &http.Server{
		Addr:              ":443",
		Handler:           handler,
		TLSConfig:         mtlsConfig,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go serve("http", plain.ListenAndServe)
	go serve("mtls", func() error { return mtls.ListenAndServeTLS("", "") })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = plain.Shutdown(shutdownCtx)
	_ = mtls.Shutdown(shutdownCtx)
}

func serve(name string, run func() error) {
	slog.Info("A6 hostile target starting", "listener", name)
	if err := run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("A6 hostile target stopped", "listener", name, "error", err)
		os.Exit(1)
	}
}

func mtlsServerConfig() (*tls.Config, error) {
	now := time.Now()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gear-a6-local-ca"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "gear-policy.gear-system.svc"},
		DNSNames: []string{
			"gear-policy",
			"a6-gear-policy",
			"a6-gear-policy.gear-system",
			"a6-gear-policy.gear-system.svc",
			"a6-gear-policy.gear-system.svc.cluster.local",
		},
		NotBefore:   now.Add(-time.Hour),
		NotAfter:    now.Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
