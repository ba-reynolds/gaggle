package metrics

import (
	"context"
	"log/slog"
	"time"
)

// SamplerStore is the persistence side the Sampler needs.
type SamplerStore interface {
	InsertHostSample(ctx context.Context, h *HostStats) error
	PrunePageViews(ctx context.Context, cutoff time.Time) error
	PruneHostSamples(ctx context.Context, cutoff time.Time) error
}

// Sampler periodically records host stats so the metrics dashboard can show
// history, and prunes old metrics rows so the tables stay bounded. It runs
// for the life of the api process — sampling is therefore independent of
// anyone actually opening the dashboard, so history accrues 24/7 from deploy.
type Sampler struct {
	store       SamplerStore
	logger      *slog.Logger
	sampleEvery time.Duration
	retention   time.Duration
}

// NewSampler creates a Sampler. sampleEvery is the cadence of host snapshots
// and retention how long page_views/host_metrics_samples rows are kept.
func NewSampler(store SamplerStore, logger *slog.Logger, sampleEvery, retention time.Duration) *Sampler {
	return &Sampler{store: store, logger: logger, sampleEvery: sampleEvery, retention: retention}
}

// Run samples immediately, then every sampleEvery, until ctx is cancelled.
// A prune of old rows runs on an hourly tick.
func (s *Sampler) Run(ctx context.Context) {
	s.logger.Info("metrics sampler started",
		"sample_every", s.sampleEvery.String(),
		"retention", s.retention.String())

	sampleTicker := time.NewTicker(s.sampleEvery)
	pruneTicker := time.NewTicker(time.Hour)
	defer func() {
		sampleTicker.Stop()
		pruneTicker.Stop()
		s.logger.Info("metrics sampler stopped")
	}()

	s.sample(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-pruneTicker.C:
			s.prune(ctx)
		case <-sampleTicker.C:
			s.sample(ctx)
		}
	}
}

func (s *Sampler) sample(ctx context.Context) {
	stats, err := ReadHostStats()
	if err != nil {
		s.logger.Warn("metrics sampler: failed to read host stats", "error", err)
		return
	}
	if err := s.store.InsertHostSample(ctx, stats); err != nil {
		s.logger.Warn("metrics sampler: failed to insert host sample", "error", err)
	}
}

func (s *Sampler) prune(ctx context.Context) {
	cutoff := time.Now().Add(-s.retention)
	if err := s.store.PrunePageViews(ctx, cutoff); err != nil {
		s.logger.Warn("metrics sampler: failed to prune page_views", "error", err)
	}
	if err := s.store.PruneHostSamples(ctx, cutoff); err != nil {
		s.logger.Warn("metrics sampler: failed to prune host samples", "error", err)
	}
}