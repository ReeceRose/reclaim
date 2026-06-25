package api

import (
	"strconv"
	"strings"
	"sync"

	"reclaim/internal/store"
)

const maxCandidateCacheEntries = 64

// candidateCache holds derived candidate views (grouped summaries and season
// episodes). Entries are keyed by filter/query params and cleared wholesale on
// invalidation — candidate eligibility changes are infrequent relative to reads.
type candidateCache struct {
	mu       sync.RWMutex
	grouped  map[string][]seriesSummary
	episodes map[string][]episodeDTO
}

func newCandidateCache() *candidateCache {
	return &candidateCache{
		grouped:  make(map[string][]seriesSummary),
		episodes: make(map[string][]episodeDTO),
	}
}

// InvalidateCandidates drops all cached candidate views. Call after any write
// that can change which files are candidates or their savings/path metadata.
func (s *Server) InvalidateCandidates() {
	if s == nil || s.candCache == nil {
		return
	}
	s.candCache.invalidate()
}

func (c *candidateCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.grouped = make(map[string][]seriesSummary)
	c.episodes = make(map[string][]episodeDTO)
}

func filterCacheKey(f store.CandidateFilter) string {
	var b strings.Builder
	b.Grow(len(f.LibraryType) + len(f.VideoCodec) + len(f.ResolutionBand) + len(f.Search) + 4)
	b.WriteString(f.LibraryType)
	b.WriteByte(0)
	b.WriteString(f.VideoCodec)
	b.WriteByte(0)
	b.WriteString(f.ResolutionBand)
	b.WriteByte(0)
	b.WriteString(f.Search)
	return b.String()
}

func seasonEpisodesCacheKey(f store.CandidateFilter, series string, season int) string {
	return filterCacheKey(f) + "\x00" + series + "\x00" + strconv.Itoa(season)
}

func (c *candidateCache) getGrouped(key string) ([]seriesSummary, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.grouped[key]
	return v, ok
}

func (c *candidateCache) putGrouped(key string, v []seriesSummary) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.grouped) >= maxCandidateCacheEntries {
		c.grouped = make(map[string][]seriesSummary)
		c.episodes = make(map[string][]episodeDTO)
	}
	c.grouped[key] = v
}

func (c *candidateCache) getEpisodes(key string) ([]episodeDTO, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.episodes[key]
	return v, ok
}

func (c *candidateCache) putEpisodes(key string, v []episodeDTO) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.episodes) >= maxCandidateCacheEntries {
		c.grouped = make(map[string][]seriesSummary)
		c.episodes = make(map[string][]episodeDTO)
	}
	c.episodes[key] = v
}
