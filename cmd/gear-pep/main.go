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

	"gear/internal/pepcore"
)

func main() {
	listen := flag.String("listen", getenv("GEAR_PEP_LISTEN", pepcore.LoopbackListenAddress), "loopback listen address")
	policyURL := flag.String("policy-url", getenv("GEAR_POLICY_URL", "http://gear-policy.gear-system.svc.cluster.local:8080"), "gear-policy base URL")
	flag.Parse()

	active, err := pepcore.ActiveActionFromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}
	if !active.Available() {
		slog.Warn("gear-pep started without active governed action; action endpoints fail closed")
	}

	effects := pepcore.NewPolicyEffectMediator(pepcore.NewHTTPPolicyClient(*policyURL))
	server, err := pepcore.NewLoopbackServer(*listen, pepcore.NewLoopbackHandler(pepcore.LoopbackConfig{
		ActiveAction: active,
		Effects:      effects,
	}))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		slog.Info("gear-pep loopback starting", "addr", *listen, "policyURL", *policyURL)
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

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
