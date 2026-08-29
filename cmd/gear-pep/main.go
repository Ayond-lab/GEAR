package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gear/internal/exectoken"
	"gear/internal/mtls"
	"gear/internal/pepcore"
	"gear/internal/policy"
)

func main() {
	listen := flag.String("listen", getenv("GEAR_PEP_LISTEN", pepcore.LoopbackListenAddress), "loopback listen address")
	policyURL := flag.String("policy-url", getenv("GEAR_POLICY_URL", "http://gear-policy.gear-system.svc.cluster.local:8080"), "gear-policy base URL")
	auditURL := flag.String("audit-url", getenv("GEAR_AUDIT_URL", "http://gear-audit.gear-system.svc.cluster.local:8080"), "gear-audit base URL")
	policyClientCert := flag.String("policy-client-cert", os.Getenv("GEAR_POLICY_CLIENT_CERT"), "gear-policy client certificate")
	policyClientKey := flag.String("policy-client-key", os.Getenv("GEAR_POLICY_CLIENT_KEY"), "gear-policy client key")
	policyCA := flag.String("policy-ca", os.Getenv("GEAR_POLICY_CA"), "gear-policy CA certificate")
	execTokenPublicKey := flag.String("exec-token-public-key", os.Getenv("GEAR_EXEC_TOKEN_PUBLIC_KEY"), "execution-token public key")
	allowedScopes := flag.String("allowed-scopes", getenv("GEAR_ALLOWED_SCOPES", "candidate-record:write"), "comma-separated connector:scope values PEP may execute")
	flag.Parse()

	active, err := pepcore.ActiveActionFromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	if !active.Available() {
		slog.Warn("gear-pep started without active governed action; action endpoints fail closed")
	}

	policyClient := pepcore.NewHTTPPolicyClient(*policyURL)
	if mtls.AllFilesExist(*policyClientCert, *policyClientKey, *policyCA) {
		client, err := mtls.Client(*policyClientCert, *policyClientKey, *policyCA)
		if err != nil {
			log.Fatal(err)
		}
		policyClient.Client = client
	} else if anySet(*policyClientCert, *policyClientKey, *policyCA) {
		log.Fatal("gear-policy mTLS configuration is incomplete")
	}
	verifier := pepcore.NewTokenVerifier()
	if *execTokenPublicKey != "" {
		key, err := exectoken.LoadPublicKey(*execTokenPublicKey)
		if err != nil {
			log.Fatal(err)
		}
		verifier = pepcore.NewTokenVerifierWithKey(key)
	}

	effects := pepcore.NewPolicyEffectMediator(policyClient).
		WithAudit(policy.NewHTTPAuditClient(*auditURL)).
		WithTokenVerifier(verifier).
		WithAllowedScopes(pepcore.ParseAllowedScopes(*allowedScopes))
	server, err := pepcore.NewLoopbackServer(*listen, pepcore.NewLoopbackHandler(pepcore.LoopbackConfig{
		ActiveAction: active,
		Effects:      effects,
	}))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		slog.Info("gear-pep loopback starting", "addr", *listen, "policyURL", *policyURL, "auditURL", *auditURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}

func anySet(values ...string) bool {
	for _, value := range values {
		if value != "" {
			return true
		}
	}
	return false
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
