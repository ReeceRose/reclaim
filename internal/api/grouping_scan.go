package api

import (
	"context"
	"sort"

	"reclaim/internal/media"
	"reclaim/internal/store"
)

const tvScanPageSize = 200

func (s *Server) scanTVLibraryFiles(ctx context.Context, filter store.FileFilter, fn func([]store.MediaFile, map[int64]store.CandidateState) error) error {
	offset := 0
	for {
		files, err := s.store.Media.Files(ctx, store.FileQuery{
			Sort:   store.SortPathAsc,
			Filter: filter,
			Limit:  tvScanPageSize,
			Offset: offset,
		})
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		states, err := s.store.Media.CandidateStates(ctx, files)
		if err != nil {
			return err
		}
		if err := fn(files, states); err != nil {
			return err
		}
		offset += len(files)
		if len(files) < tvScanPageSize {
			return nil
		}
	}
}

func (s *Server) scanTVCandidates(ctx context.Context, filter store.CandidateFilter, fn func([]store.MediaFile) error) error {
	offset := 0
	for {
		files, err := s.store.Media.Candidates(ctx, store.CandidateQuery{
			Sort:   store.SortSavingsDesc,
			Filter: filter,
			Limit:  tvScanPageSize,
			Offset: offset,
		})
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		if err := fn(files); err != nil {
			return err
		}
		offset += len(files)
		if len(files) < tvScanPageSize {
			return nil
		}
	}
}

type candidateTVAccumulator struct {
	bySeries map[string]*candidateSeriesAcc
}

type candidateSeriesAcc struct {
	title   string
	lib     string
	seasons map[int]*candidateSeasonAcc
	order   []int
	savings int64
}

type candidateSeasonAcc struct {
	season int
	ids    []int64
	bytes  int64
	saving int64
}

func newCandidateTVAccumulator() *candidateTVAccumulator {
	return &candidateTVAccumulator{bySeries: map[string]*candidateSeriesAcc{}}
}

func (a *candidateTVAccumulator) add(files []store.MediaFile, tvPath string) {
	for i := range files {
		f := &files[i]
		title, season, _ := media.ParseTVInfo(f.Path, tvPath)
		acc, ok := a.bySeries[title]
		if !ok {
			acc = &candidateSeriesAcc{title: title, lib: f.LibraryType, seasons: map[int]*candidateSeasonAcc{}}
			a.bySeries[title] = acc
		}
		acc.savings += f.PredictedSavingsBytes

		sa, ok := acc.seasons[season]
		if !ok {
			sa = &candidateSeasonAcc{season: season}
			acc.seasons[season] = sa
			acc.order = append(acc.order, season)
		}
		sa.ids = append(sa.ids, f.ID)
		sa.bytes += f.SizeBytes
		sa.saving += f.PredictedSavingsBytes
	}
}

func (a *candidateTVAccumulator) summaries(seasonFiles map[seasonKey]int, seriesFiles map[string]int) []seriesSummary {
	seriesOrder := make([]string, 0, len(a.bySeries))
	for title := range a.bySeries {
		seriesOrder = append(seriesOrder, title)
	}
	sort.SliceStable(seriesOrder, func(i, j int) bool {
		return a.bySeries[seriesOrder[i]].savings > a.bySeries[seriesOrder[j]].savings
	})

	out := make([]seriesSummary, 0, len(seriesOrder))
	for _, title := range seriesOrder {
		acc := a.bySeries[title]
		sg := seriesSummary{Title: acc.title, LibraryType: acc.lib}

		sort.Ints(acc.order)
		for _, sn := range acc.order {
			sa := acc.seasons[sn]
			fileCount := seasonFiles[seasonKey{acc.title, sn}]
			if fileCount == 0 {
				fileCount = len(sa.ids)
			}
			sg.Seasons = append(sg.Seasons, seasonSummary{
				Season:                sn,
				FileCount:             fileCount,
				CandidateCount:        len(sa.ids),
				TotalBytes:            sa.bytes,
				PredictedSavingsBytes: sa.saving,
				EpisodeIDs:            sa.ids,
			})
			sg.CandidateCount += len(sa.ids)
			sg.TotalBytes += sa.bytes
			sg.PredictedSavingsBytes += sa.saving
		}
		sg.FileCount = seriesFiles[title]
		if sg.FileCount == 0 {
			sg.FileCount = sg.CandidateCount
		}
		sg.SeasonCount = len(sg.Seasons)
		out = append(out, sg)
	}
	return out
}

type seasonKey struct {
	title  string
	season int
}

func (s *Server) buildTVCandidateSummaries(ctx context.Context, filter store.CandidateFilter) ([]seriesSummary, error) {
	seasonFiles := make(map[seasonKey]int)
	seriesFiles := make(map[string]int)
	if err := s.scanTVLibraryFiles(ctx, store.FileFilter{
		LibraryType: store.LibraryTypeTV,
		Search:      filter.Search,
	}, func(files []store.MediaFile, _ map[int64]store.CandidateState) error {
		for i := range files {
			f := &files[i]
			if f.Status != store.MediaStatusActive {
				continue
			}
			title, season, _ := media.ParseTVInfo(f.Path, s.tvPath)
			seasonFiles[seasonKey{title, season}]++
			seriesFiles[title]++
		}
		return nil
	}); err != nil {
		return nil, err
	}

	tvFilter := filter
	tvFilter.LibraryType = store.LibraryTypeTV
	acc := newCandidateTVAccumulator()
	if err := s.scanTVCandidates(ctx, tvFilter, func(files []store.MediaFile) error {
		acc.add(files, s.tvPath)
		return nil
	}); err != nil {
		return nil, err
	}
	return acc.summaries(seasonFiles, seriesFiles), nil
}
