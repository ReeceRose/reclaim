package store

import (
	"context"
	"maps"
	"strings"
	"sync"

	"reclaim/internal/media"
)

// SavingsModel is the single source of predicted_savings_bytes. It holds the
// learned output/original ratio per source codec in memory so the scanner can
// price every probe without a ledger query, and falls back to the seed table
// for codecs that have not yet reached LearnedRatioMinSamples.
//
// Before this existed only the worker's post-encode hook applied learned
// ratios, and only to the codec it had just encoded, while every insert and
// re-probe went back to the seed. New arrivals were ranked on the seed until
// the next encode of their codec happened to land.
type SavingsModel struct {
	jobs  *Jobs
	media *Media
	stats *Stats

	mu      sync.RWMutex
	learned map[string]LearnedRatio
}

// Predict returns the savings estimate for a file, preferring this instance's
// learned ratio for the codec over the seed.
func (m *SavingsModel) Predict(videoCodec *string, isAlreadyHEVC bool, sizeBytes int64) int64 {
	if videoCodec != nil {
		m.mu.RLock()
		lr, ok := m.learned[strings.ToLower(*videoCodec)]
		m.mu.RUnlock()
		if ok {
			return media.SavingsForRatio(lr.Ratio, isAlreadyHEVC, sizeBytes)
		}
	}
	return media.PredictedSavingsBytes(videoCodec, isAlreadyHEVC, sizeBytes)
}

// Learned returns a copy of the current learned ratios keyed by lowercase codec.
func (m *SavingsModel) Learned() map[string]LearnedRatio {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return maps.Clone(m.learned)
}

// Refresh reloads the learned ratios from the ledger and rewrites the stored
// prediction of every active file whose codec has one, then reconciles
// library_stats if anything moved. It runs at boot and after each completed
// encode.
//
// The cache is swapped before the rewrite so a probe racing the refresh prices
// on the new ratio; one that read the old ratio just before the swap is
// corrected by the next refresh.
func (m *SavingsModel) Refresh(ctx context.Context) (int64, error) {
	learned, err := m.jobs.LearnedRatios(ctx, LearnedRatioMinSamples)
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	m.learned = learned
	m.mu.Unlock()

	var total int64
	for codec, lr := range learned {
		n, err := m.media.UpdatePredictedSavingsByCodec(ctx, codec, lr.Ratio)
		if err != nil {
			return total, err
		}
		total += n
	}
	if total > 0 {
		if err := m.stats.Recompute(ctx); err != nil {
			return total, err
		}
	}
	return total, nil
}
