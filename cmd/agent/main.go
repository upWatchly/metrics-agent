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
	logLevel, err := log.ParseLevel(os.Getenv("UW_LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}
	log.SetLevel(logLevel)

	isService, err := inWindowsService()
	if err != nil {
		log.WithError(err).Fatal("failed to determine windows service mode")
	}

	// Redirect logging to the service log file before the first log line, so a
	// service (which has no console) captures startup output too.
	if isService {
		configureServiceLogging()
	}

	log.WithFields(log.Fields{
		"version": version,
		"commit":  commit,
		"service": isService,
	}).Info("metrics-agent starting")

	// Under the Windows SCM there is no console to inherit and no signal
	// delivery: hand off to the service control handler, which drives runAgent
	// itself. On every other platform (and the .exe run from a console) we fall
	// through to the foreground path below.
	if isService {
		if err := runWindowsService(); err != nil {
			log.WithError(err).Fatal("windows service failed")
		}
		return
	}

	// Foreground: translate Ctrl-C / SIGTERM into context cancellation.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runAgent(ctx); err != nil {
		log.WithError(err).Fatal("agent exited with error")
	}

	log.Info("metrics-agent stopped")
}

// runAgent is the platform-agnostic entry point shared by the console path and
// the Windows service control handler. It returns an error only for a startup
// failure (e.g. bad config); a normal shutdown via ctx cancellation returns nil.
func runAgent(ctx context.Context) error {
	cfg, err := agent.LoadConfig()
	if err != nil {
		return err
	}

	if os.Getenv("UW_PPROF") == "true" {
		go func() {
			// Bind loopback only — the pprof handlers expose heap/goroutine
			// dumps and a CPU-profile DoS vector; they must not be reachable
			// off-host.
			log.Info("pprof listening on 127.0.0.1:6060")
			if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
				log.WithError(err).Warn("pprof server failed")
			}
		}()
	}

	return agent.New(cfg, version).Run(ctx)
}
