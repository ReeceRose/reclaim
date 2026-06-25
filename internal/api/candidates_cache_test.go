package api

import (
	"testing"

	"reclaim/internal/store"
)

func TestCandidateCacheGrouped(t *testing.T) {
	cache := newCandidateCache()
	filter := store.CandidateFilter{LibraryType: "tv", VideoCodec: "h264"}
	key := filterCacheKey(filter)

	if _, ok := cache.getGrouped(key); ok {
		t.Fatal("expected cache miss")
	}

	want := []seriesSummary{{Title: "Show A", CandidateCount: 2}}
	cache.putGrouped(key, want)

	got, ok := cache.getGrouped(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 1 || got[0].Title != "Show A" {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCandidateCacheInvalidateClearsEntries(t *testing.T) {
	srv := &Server{candCache: newCandidateCache()}
	filter := store.CandidateFilter{LibraryType: "tv"}
	key := filterCacheKey(filter)

	srv.candCache.putGrouped(key, []seriesSummary{{Title: "Show A"}})
	srv.candCache.putEpisodes(seasonEpisodesCacheKey(filter, "Show A", 1), []episodeDTO{{Season: 1}})

	srv.InvalidateCandidates()

	if _, ok := srv.candCache.getGrouped(key); ok {
		t.Fatal("grouped cache should be empty after invalidation")
	}
	if _, ok := srv.candCache.getEpisodes(seasonEpisodesCacheKey(filter, "Show A", 1)); ok {
		t.Fatal("episodes cache should be empty after invalidation")
	}
}

func TestCandidateCacheMaxEntriesResets(t *testing.T) {
	cache := newCandidateCache()
	for i := range maxCandidateCacheEntries {
		cache.putGrouped(filterCacheKey(store.CandidateFilter{Search: string(rune('a' + i))}), nil)
	}
	cache.putGrouped("overflow", []seriesSummary{{Title: "overflow"}})
	if len(cache.grouped) != 1 {
		t.Fatalf("expected cache reset on overflow, got %d entries", len(cache.grouped))
	}
}
