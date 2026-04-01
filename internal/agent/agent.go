package agent

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/upwatchly/metrics-agent/internal/client"
	"github.com/upwatchly/metrics-agent/internal/collector"
)

const (
	defaultReportInterval = 60 // seconds

	minReportInterval = 10 * time.Second
	maxReportInterval = 3600 * time.Second
	minLiveInterval   = 1 * time.Second
	maxLiveInterval   = 60 * time.Second
)

// Agent orchestrates metric collection and reporting.
type Agent struct {
	client    *client.Client
	collector *collector.Collector

	// Mutable state protected by mu
	mu             sync.RWMutex
	reportInterval time.Duration
	liveMode       bool
	serverID       string
	organizationID string
	slowCancel     context.CancelFunc

	// Latest collected metrics snapshot
	latestMetrics *client.MetricsReport
	metricsMu     sync.RWMutex

	// Signal channels
	wakeCollect chan struct{}
	firstReady  chan struct{} // closed when first metrics collected
}

// New creates a new Agent.
func New(cfg *Config) *Agent {
	return &Agent{
		client: client.New(cfg.APIEndpoint, cfg.APIKey, cfg.DisableKeepAlive),

		collector:      collector.New(),
		reportInterval: defaultReportInterval * time.Second,
		wakeCollect:    make(chan struct{}, 1),
		firstReady:     make(chan struct{}),
	}
}

func (a *Agent) getLiveMode() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.liveMode
}

func (a *Agent) getReportInterval() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.reportInterval
}

// Run starts the main reporting loop. Blocks until ctx is cancelled.
func (a *Agent) Run(ctx context.Context) {
	log.Info("agent started")

	// Start background metrics collection
	go a.collectLoop(ctx)

	// Wait for first metrics to be collected (with timeout)
	select {
	case <-ctx.Done():
		return
	case <-a.firstReady:
	case <-time.After(2 * time.Minute):
		log.Fatal("failed to collect initial metrics within 2 minutes — check system permissions")
	}

	// First send immediately
	a.send(ctx)

	ticker := time.NewTicker(a.getReportInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("agent stopping")
			return
		case <-ticker.C:
			prevInterval := a.getReportInterval()
			a.send(ctx)
			newInterval := a.getReportInterval()
			if newInterval != prevInterval {
				ticker.Reset(newInterval)
			}
		}
	}
}

// collectLoop continuously collects metrics in the background.
func (a *Agent) collectLoop(ctx context.Context) {
	firstOnce := sync.Once{}

	for {
		metrics, err := a.collector.Collect(ctx, a.getLiveMode())
		if err != nil {
			log.WithError(err).Error("failed to collect metrics")
		} else {
			a.metricsMu.Lock()
			a.latestMetrics = metrics
			a.metricsMu.Unlock()
			firstOnce.Do(func() { close(a.firstReady) })
		}

		timer := time.NewTimer(a.getReportInterval() / 2)
		if !a.getLiveMode() {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-a.wakeCollect:
				timer.Stop()
			case <-timer.C:
			}
		} else {
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (a *Agent) send(ctx context.Context) {
	a.metricsMu.RLock()
	orig := a.latestMetrics
	a.metricsMu.RUnlock()

	if orig == nil {
		return
	}

	// Copy to avoid mutating the shared object
	metrics := *orig

	a.mu.RLock()
	metrics.ServerID = a.serverID
	metrics.OrganizationID = a.organizationID
	live := a.liveMode
	a.mu.RUnlock()

	log.WithFields(log.Fields{
		"cpu":      metrics.CPU,
		"mem":      metrics.Memory,
		"disks":    len(metrics.Disks),
		"netIn":    metrics.Network.InBytesPerSec,
		"netOut":   metrics.Network.OutBytesPerSec,
		"liveMode": live,
	}).Debug("sending metrics")

	config, err := a.client.SendMetrics(ctx, &metrics)
	if err != nil {
		log.WithError(err).Error("failed to send metrics")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Cache server identity
	a.serverID = config.ServerID
	a.organizationID = config.OrganizationID

	// Apply live mode
	if config.LiveMode && config.LiveInterval > 0 {
		if !a.liveMode {
			log.WithField("interval", config.LiveInterval).Info("entering live mode")
			slowCtx, slowCancel := context.WithCancel(ctx)
			a.slowCancel = slowCancel
			a.collector.StartSlowCollector(slowCtx)
			select {
			case a.wakeCollect <- struct{}{}:
			default:
			}
		}
		a.liveMode = true
		a.reportInterval = clampDuration(
			time.Duration(config.LiveInterval)*time.Second,
			minLiveInterval, maxLiveInterval,
		)
	} else {
		if a.liveMode {
			log.Info("exiting live mode")
			if a.slowCancel != nil {
				a.slowCancel()
				a.slowCancel = nil
			}
		}
		a.liveMode = false
		if config.ReportInterval > 0 {
			a.reportInterval = clampDuration(
				time.Duration(config.ReportInterval)*time.Second,
				minReportInterval, maxReportInterval,
			)
		} else {
			a.reportInterval = defaultReportInterval * time.Second
		}
	}

	log.Debug("metrics reported successfully")
}

func clampDuration(d, min, max time.Duration) time.Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}
