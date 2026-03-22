package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/upwatchly/metrics-agent/internal/agent"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetLevel(log.DebugLevel)

	log.WithFields(log.Fields{
		"version": version,
		"commit":  commit,
	}).Info("metrics-agent starting")

	cfg, err := agent.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("failed to load config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.WithField("signal", sig.String()).Info("received shutdown signal")
		cancel()
	}()

	if os.Getenv("UW_PPROF") == "true" {
		go func() {
			log.Info("pprof listening on :6060")
			if err := http.ListenAndServe(":6060", nil); err != nil {
				log.WithError(err).Warn("pprof server failed")
			}
		}()
	}

	a := agent.New(cfg)
	a.Run(ctx)

	log.Info("metrics-agent stopped")
}
