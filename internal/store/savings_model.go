package store

import (
	"context"
	"errors"
	"maps"
	"strings"
	"sync"

	"reclaim/internal/media"
)

// SavingsModel is the single source of predicted_savings_bytes. It holds the
// learned output/original ratio per target and source codec in memory so the
// scanner can price every probe without a ledger query, and falls back to the
// seed table for pairs that have not yet reached LearnedRatioMinSamples.
//
// A file carries one stored prediction, so the library is priced against one
// target: the default profile's codec. Queueing with a profile that encodes to
// a different codec asks PredictFor for that target instead.
//
// Before this existed only the worker's post-encode hook applied learned
// ratios, and only to the codec it had just encoded, while every insert and
// re-probe went back to the seed. New arrivals were ranked on the seed until
// the next encode of their codec happened to land.
type SavingsModel struct {
	jobs     *Jobs
	media    *Media
	stats    *Stats
	profiles *Profiles

	mu      sync.RWMutex
	target  media.TargetCodec
	learned map[media.TargetCodec]map[string]LearnedRatio
}

// Target returns the codec stored predictions are priced against.
func (m *SavingsModel) Target() media.TargetCodec {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.target == "" {
		return media.DefaultTargetCodec
	}
	return m.target
}

// Predict returns the savings estimate for a file against the library's
// target codec, preferring this instance's learned ratio over the seed.
func (m *SavingsModel) Predict(videoCodec *string, isEfficient bool, sizeBytes int64) int64 {
	return m.PredictFor(m.Target(), videoCodec, isEfficient, sizeBytes)
}

// PredictFor returns the savings estimate for re-encoding a file to target.
func (m *SavingsModel) PredictFor(target media.TargetCodec, videoCodec *string, isEfficient bool, sizeBytes int64) int64 {
	if lr, ok := m.learnedFor(target, videoCodec); ok {
		return media.SavingsForRatio(lr.Ratio, isEfficient, sizeBytes)
	}
	return media.PredictedSavingsBytes(target, videoCodec, isEfficient, sizeBytes)
}

func (m *SavingsModel) learnedFor(target media.TargetCodec, videoCodec *string) (LearnedRatio, bool) {
	if videoCodec == nil {
		return LearnedRatio{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	lr, ok := m.learned[target][strings.ToLower(*videoCodec)]
	return lr, ok
}

// Learned returns a copy of the learned ratios for the library's target codec,
// keyed by lowercase source codec.
func (m *SavingsModel) Learned() map[string]LearnedRatio {
	target := m.Target()
	m.mu.RLock()
	defer m.mu.RUnlock()
	return maps.Clone(m.learned[target])
}

// Refresh re-reads the target codec from the default profile and the learned
// ratios from the ledger, rewrites the stored prediction of every priced codec
// whose value moved, and reconciles library_stats if anything did. It runs at
// boot, after each completed encode, and after any profile change, since
// changing the default profile's codec reprices the whole library.
//
// The cache is swapped before the rewrite so a probe racing the refresh prices
// on the new ratio; one that read the old ratio just before the swap is
// corrected by the next refresh.
func (m *SavingsModel) Refresh(ctx context.Context) (int64, error) {
	target := media.DefaultTargetCodec
	if def, err := m.profiles.GetDefault(ctx); err == nil {
		target = media.NormalizeTargetCodec(def.Codec)
	} else if !errors.Is(err, ErrNotFound) {
		return 0, err
	}
	if _, ok := media.EncoderFor(target); !ok {
		target = media.DefaultTargetCodec
	}

	learned := make(map[media.TargetCodec]map[string]LearnedRatio)
	for _, t := range media.TargetCodecs() {
		lr, err := m.jobs.LearnedRatios(ctx, t, LearnedRatioMinSamples)
		if err != nil {
			return 0, err
		}
		learned[t] = lr
	}
	m.mu.Lock()
	m.target = target
	m.learned = learned
	m.mu.Unlock()

	codecs, err := m.media.PricedCodecs(ctx)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, codec := range codecs {
		ratio, _ := media.RatioFor(target, &codec)
		if lr, ok := learned[target][codec]; ok {
			ratio = lr.Ratio
		}
		n, err := m.media.UpdatePredictedSavingsByCodec(ctx, codec, ratio)
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
