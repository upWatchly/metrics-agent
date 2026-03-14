package agent

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/upwatchly/metrics-agent/internal/client"
	"github.com/upwatchly/metrics-agent/internal/collector"
)

const defaultReportInterval = 60 // seconds

// Agent orchestrates metric collection and reporting.
type Agent struct {
	client         *client.Client
	collector      *collector.Collector
	reportInterval time.Duration
}

// New creates a new Agent.
func New(cfg *Config) *Agent {
	return &Agent{
		client:         client.New(cfg.APIEndpoint, cfg.APIKey),
		collector:      collector.New(),
		reportInterval: defaultReportInterval * time.Second,
	}
}

// Run starts the main reporting loop. Blocks until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) {
	log.Info("agent started")

	// First report immediately
	a.report(ctx)

	ticker := time.NewTicker(a.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("agent stopping")
			return
		case <-ticker.C:
			prevInterval := a.reportInterval
			a.report(ctx)
			if a.reportInterval != prevInterval {
				ticker.Reset(a.reportInterval)
			}
		}
	}
}

func (a *Agent) report(ctx context.Context) {
	reportCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	metrics, err := a.collector.Collect(reportCtx)
	if err != nil {
		log.WithError(err).Error("failed to collect metrics")
		return
	}

	log.WithFields(log.Fields{
		"cpu":    metrics.CPU,
		"mem":    metrics.Memory,
		"disks":  len(metrics.Disks),
		"netIn":  metrics.Network.InBytesPerSec,
		"netOut": metrics.Network.OutBytesPerSec,
	}).Debug("collected metrics")

	config, err := a.client.SendMetrics(reportCtx, metrics)
	if err != nil {
		log.WithError(err).Error("failed to send metrics")
		return
	}

	// Apply new report interval from server config
	if config.ReportInterval > 0 {
		newInterval := time.Duration(config.ReportInterval) * time.Second
		if newInterval != a.reportInterval {
			log.WithField("interval", config.ReportInterval).Info("report interval updated")
			a.reportInterval = newInterval
		}
	}

	log.Debug("metrics reported successfully")
}
