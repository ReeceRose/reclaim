package notify

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"reclaim/internal/store"
)

// Season is one season's contribution to a TV title's batch.
type Season struct {
	Number       int   `json:"season"`
	Count        int   `json:"count"`
	SizeBytes    int64 `json:"size_bytes"`
	SavingsBytes int64 `json:"savings_bytes"`
}

// TitleStat is one title's line in a rollup.
type TitleStat struct {
	Title        string `json:"title"`
	LibraryType  string `json:"library_type"`
	Count        int    `json:"count"`
	SizeBytes    int64  `json:"size_bytes"`
	SavingsBytes int64  `json:"savings_bytes"`
}

// Summary is one notification. Normally that is a single title — one TV series
// (across however many of its seasons arrived) or one movie — because mixing
// unrelated shows into one message makes it unactionable: "Severance · Season 3
// — 9 new re-encode candidates" tells you what to do, "14 new candidates across
// your library" does not.
//
// The exception is a rollup, where Title is empty: see RollUp.
type Summary struct {
	Title        string `json:"title,omitempty"`
	LibraryType  string `json:"library_type,omitempty"`
	Count        int    `json:"count"`
	Titles       int    `json:"titles"`
	SizeBytes    int64  `json:"size_bytes"`
	SavingsBytes int64  `json:"predicted_savings_bytes"`

	// Seasons is set on a TV title, ascending. Files whose season could not be
	// parsed are counted in Count but absent here.
	Seasons []Season `json:"seasons,omitempty"`

	// Rollup lists the titles a rollup covers, capped at maxRollupTitles.
	Rollup []TitleStat `json:"rollup,omitempty"`
}

// maxRollupTitles bounds the itemised list in a rollup body. A bulk import can
// produce hundreds of titles, and no chat client renders that well.
const maxRollupTitles = 10

// IsRollup reports whether this summary covers many titles rather than one.
func (s Summary) IsRollup() bool { return s.Title == "" }

// Split partitions candidate rows into one summary per title, ranked by
// predicted savings. A TV series is one summary no matter how many episodes or
// seasons of it arrived; every movie is its own.
func Split(files []store.MediaFile) []Summary {
	type bucket struct {
		summary Summary
		seasons map[int]*Season
	}

	order := make([]string, 0, 8)
	byTitle := make(map[string]*bucket)

	for i := range files {
		f := &files[i]
		title := titleOf(f)

		b, ok := byTitle[title]
		if !ok {
			b = &bucket{
				summary: Summary{Title: title, LibraryType: f.LibraryType, Titles: 1},
				seasons: make(map[int]*Season),
			}
			byTitle[title] = b
			order = append(order, title)
		}

		b.summary.Count++
		b.summary.SizeBytes += f.SizeBytes
		b.summary.SavingsBytes += f.PredictedSavingsBytes

		if f.LibraryType == store.LibraryTypeTV && f.SeasonNumber != nil && *f.SeasonNumber > 0 {
			num := *f.SeasonNumber
			se, ok := b.seasons[num]
			if !ok {
				se = &Season{Number: num}
				b.seasons[num] = se
			}
			se.Count++
			se.SizeBytes += f.SizeBytes
			se.SavingsBytes += f.PredictedSavingsBytes
		}
	}

	out := make([]Summary, 0, len(order))
	for _, title := range order {
		b := byTitle[title]
		for _, se := range b.seasons {
			b.summary.Seasons = append(b.summary.Seasons, *se)
		}
		sort.Slice(b.summary.Seasons, func(i, j int) bool {
			return b.summary.Seasons[i].Number < b.summary.Seasons[j].Number
		})
		out = append(out, b.summary)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SavingsBytes != out[j].SavingsBytes {
			return out[i].SavingsBytes > out[j].SavingsBytes
		}
		return out[i].Title < out[j].Title
	})
	return out
}

// RollUp collapses many per-title summaries into one. It is the safety valve for
// a bulk import: one notification saying 200 files arrived is far better than
// 200 notifications, and better than a webhook receiver rate-limiting us.
func RollUp(summaries []Summary) Summary {
	out := Summary{Titles: len(summaries)}
	for _, s := range summaries {
		out.Count += s.Count
		out.SizeBytes += s.SizeBytes
		out.SavingsBytes += s.SavingsBytes
		if len(out.Rollup) < maxRollupTitles {
			out.Rollup = append(out.Rollup, TitleStat{
				Title:        s.Title,
				LibraryType:  s.LibraryType,
				Count:        s.Count,
				SizeBytes:    s.SizeBytes,
				SavingsBytes: s.SavingsBytes,
			})
		}
	}
	return out
}

// titleOf names the thing a file belongs to: the series for a TV episode, and
// the filename without its extension otherwise — which for a Plex-shaped movie
// library is already the title. A TV file whose series could not be parsed falls
// back to the same filename rule, so it becomes its own title rather than
// silently joining an unrelated batch.
func titleOf(f *store.MediaFile) string {
	if f.LibraryType == store.LibraryTypeTV && f.SeriesTitle != nil && *f.SeriesTitle != "" {
		return *f.SeriesTitle
	}
	base := filepath.Base(f.Path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// Message is the one-line summary shown in the events feed and used as the
// notification title. It has to stand alone — the bell panel renders nothing but
// this string — so it always leads with what arrived.
func (s Summary) Message() string {
	if s.Count == 0 {
		return ""
	}
	if s.IsRollup() {
		return fmt.Sprintf("%d new re-encode candidates across %d titles · %s recoverable",
			s.Count, s.Titles, FormatBytes(s.SavingsBytes))
	}

	label, scope := s.Title, ""
	switch {
	// One season, and every file in the batch belongs to it: fold it into the
	// title, which reads better than repeating it as a scope clause.
	case len(s.Seasons) == 1 && s.Seasons[0].Count == s.Count:
		label = fmt.Sprintf("%s · Season %d", s.Title, s.Seasons[0].Number)
	case len(s.Seasons) > 1:
		scope = fmt.Sprintf(" across %d seasons", len(s.Seasons))
	}

	return fmt.Sprintf("%s — %d new re-encode %s%s · %s recoverable",
		label, s.Count, plural(s.Count, "candidate", "candidates"), scope,
		FormatBytes(s.SavingsBytes))
}

// Details is the itemised list a webhook body carries under the message. A
// single-season or movie batch has nothing to add — the message already says it
// all — so it returns nothing rather than padding the body.
func (s Summary) Details() []string {
	if s.IsRollup() {
		lines := make([]string, 0, len(s.Rollup)+1)
		for _, t := range s.Rollup {
			lines = append(lines, fmt.Sprintf("%s — %d %s · %s recoverable",
				t.Title, t.Count, plural(t.Count, "file", "files"), FormatBytes(t.SavingsBytes)))
		}
		if rest := s.Titles - len(s.Rollup); rest > 0 {
			lines = append(lines, fmt.Sprintf("…and %d more", rest))
		}
		return lines
	}

	if len(s.Seasons) < 2 {
		return nil
	}
	lines := make([]string, 0, len(s.Seasons))
	for _, se := range s.Seasons {
		lines = append(lines, fmt.Sprintf("Season %d — %d %s · %s recoverable",
			se.Number, se.Count, plural(se.Count, "file", "files"), FormatBytes(se.SavingsBytes)))
	}
	return lines
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// FormatBytes renders a byte count the way the UI does: base-1024, one decimal
// place above the KB mark.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTP"[exp])
}
