package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const resultPrefix = "A6_RESULT "

type runResult struct {
	Control   string         `json:"control"`
	StartedAt string         `json:"startedAt"`
	Attacks   []attackResult `json:"attacks"`
}

type attackResult struct {
	ID             string   `json:"id"`
	Description    string   `json:"description"`
	Target         string   `json:"target"`
	Outcome        string   `json:"outcome"`
	Reached        bool     `json:"reached"`
	HTTPStatus     int      `json:"httpStatus,omitempty"`
	Resolved       []string `json:"resolved,omitempty"`
	Error          string   `json:"error,omitempty"`
	DurationMillis int64    `json:"durationMillis"`
}

func main() {
	time.Sleep(startDelay())

	policyDNS := getenv("A6_POLICY_DNS", "a6-gear-policy.gear-system.svc.cluster.local")
	inferenceDNS := getenv("A6_INFERENCE_DNS", "a6-gear-inference.gear-system.svc.cluster.local")
	fixtureDNS := getenv("A6_FIXTURE_DNS", "a6-gear-fixture-store.gear-system.svc.cluster.local")
	policyIP := os.Getenv("A6_POLICY_IP")
	if policyIP == "" {
		policyIP = policyDNS
	}

	result := runResult{
		Control:   getenv("A6_CONTROL", "unknown"),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Attacks: []attackResult{
			httpGet("policy-dns-http", "Direct HTTP call to gear-policy by service DNS", "http://"+net.JoinHostPort(policyDNS, "8080")+"/policy"),
			httpGet("policy-podip-http", "Direct HTTP call to gear-policy by Pod IP", "http://"+net.JoinHostPort(policyIP, "8080")+"/policy"),
			httpGet("inference-dns-http", "Direct HTTP call to gear-inference by service DNS", "http://"+net.JoinHostPort(inferenceDNS, "8080")+"/inference"),
			httpGet("fixture-dns-http", "Direct HTTP call to gear-fixture-store by service DNS", "http://"+net.JoinHostPort(fixtureDNS, "8080")+"/fixture"),
			httpGet("connector-write-http", "Direct connector-like write call that should be PEP mediated", "http://"+net.JoinHostPort(fixtureDNS, "8080")+"/connector/candidate-record"),
			dnsLookup("dns-assisted-policy", "DNS lookup for a trusted service before egress", policyDNS),
			rawTCP("raw-tcp-policy", "Raw TCP connection to gear-policy Pod IP", net.JoinHostPort(policyIP, "8080")),
			tlsNoClientCert("mtls-policy-443", "mTLS attempt to gear-policy without a client certificate", net.JoinHostPort(policyIP, "443")),
		},
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal A6 result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(resultPrefix + string(data))
}

func startDelay() time.Duration {
	seconds, err := strconv.Atoi(getenv("A6_START_DELAY_SECONDS", "0"))
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func httpGet(id, description, target string) attackResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 2 * time.Second,
		}).DialContext,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return failed(id, description, target, "invalid-request", err, started)
	}

	resp, err := client.Do(req)
	if err != nil {
		return failed(id, description, target, "blocked", err, started)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))

	return attackResult{
		ID:             id,
		Description:    description,
		Target:         target,
		Outcome:        "reached",
		Reached:        true,
		HTTPStatus:     resp.StatusCode,
		DurationMillis: time.Since(started).Milliseconds(),
	}
}

func dnsLookup(id, description, host string) attackResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resolved, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return failed(id, description, host, "blocked", err, started)
	}

	return attackResult{
		ID:             id,
		Description:    description,
		Target:         host,
		Outcome:        "resolved",
		Reached:        true,
		Resolved:       resolved,
		DurationMillis: time.Since(started).Milliseconds(),
	}
}

func rawTCP(id, description, target string) attackResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", target)
	if err != nil {
		return failed(id, description, target, "blocked", err, started)
	}
	_ = conn.Close()

	return attackResult{
		ID:             id,
		Description:    description,
		Target:         target,
		Outcome:        "reached",
		Reached:        true,
		DurationMillis: time.Since(started).Milliseconds(),
	}
}

func tlsNoClientCert(id, description, target string) attackResult {
	started := time.Now()
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Lab target uses ephemeral certificates.
				MinVersion:         tls.VersionTLS12,
			},
		},
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("https://" + target + "/policy")
	if err != nil {
		outcome := "blocked"
		reached := false
		if looksLikeTLSAuthFailure(err) {
			outcome = "auth-failed"
			reached = true
		}
		return attackResult{
			ID:             id,
			Description:    description,
			Target:         target,
			Outcome:        outcome,
			Reached:        reached,
			Error:          err.Error(),
			DurationMillis: time.Since(started).Milliseconds(),
		}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))

	return attackResult{
		ID:             id,
		Description:    description,
		Target:         target,
		Outcome:        "unexpected-authorised",
		Reached:        true,
		DurationMillis: time.Since(started).Milliseconds(),
	}
}

func failed(id, description, target, outcome string, err error, started time.Time) attackResult {
	return attackResult{
		ID:             id,
		Description:    description,
		Target:         target,
		Outcome:        outcome,
		Reached:        false,
		Error:          err.Error(),
		DurationMillis: time.Since(started).Milliseconds(),
	}
}

func looksLikeTLSAuthFailure(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "certificate") ||
		strings.Contains(text, "bad certificate") ||
		strings.Contains(text, "handshake") ||
		strings.Contains(text, "remote error: tls")
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
