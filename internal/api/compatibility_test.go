package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"reclaim/internal/compatibility"
	"reclaim/internal/store"
)

func TestHandleCompatibilityProfilesListsAllBuiltins(t *testing.T) {
	_, h, _, _ := newTestServer(t, true)

	w := doReq(h, http.MethodGet, "/api/compatibility/profiles", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Profiles []compatibilityProfileDTO `json:"profiles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Profiles) != 4 {
		t.Fatalf("want 4 profiles, got %d: %+v", len(body.Profiles), body.Profiles)
	}
}

func TestHandleCompatibilityDefaultsToSettingsProfileAndExcludesDirectPlay(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)
	ctx := t.Context()

	hevc := "hevc"
	failID := insertAPIMedia(t, st, &store.MediaFile{Path: "/media/movies/fails.mkv", VideoCodec: &hevc})
	if err := st.Media.UpsertCompatibility(ctx, failID, map[string]compatibility.Verdict{
		"apple_tv_4k": {
			DirectPlayPredicted: false, RiskScore: 45,
			Reasons:           []compatibility.Reason{{Code: "video_codec_hevc", Severity: compatibility.Hard, Message: "x"}},
			RecommendedAction: compatibility.ActionManual,
		},
	}); err != nil {
		t.Fatal(err)
	}

	h264 := "h264"
	passID := insertAPIMedia(t, st, &store.MediaFile{Path: "/media/movies/passes.mp4", VideoCodec: &h264})
	if err := st.Media.UpsertCompatibility(ctx, passID, map[string]compatibility.Verdict{
		"apple_tv_4k": {DirectPlayPredicted: true, RecommendedAction: compatibility.ActionNone},
	}); err != nil {
		t.Fatal(err)
	}

	// No ?client_profile= — should silently default to settings
	// (apple_tv_4k, per migration 00010's column default) and, by default,
	// only show the file predicted NOT to direct-play.
	w := doReq(h, http.MethodGet, "/api/compatibility", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Items      []compatibilityItemDTO `json:"items"`
		TotalCount int64                  `json:"total_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TotalCount != 1 || len(body.Items) != 1 {
		t.Fatalf("want exactly 1 item (default direct_play=false filter), got total=%d items=%+v", body.TotalCount, body.Items)
	}
	if body.Items[0].ID != failID {
		t.Fatalf("want failing file %d, got %d", failID, body.Items[0].ID)
	}
	if body.Items[0].Compatibility.ClientProfile != "apple_tv_4k" {
		t.Fatalf("compatibility.client_profile = %q, want apple_tv_4k", body.Items[0].Compatibility.ClientProfile)
	}
	if len(body.Items[0].Compatibility.Reasons) != 1 || body.Items[0].Compatibility.Reasons[0].Code != "video_codec_hevc" {
		t.Fatalf("unexpected reasons: %+v", body.Items[0].Compatibility.Reasons)
	}

	// direct_play=all should return both.
	w = doReq(h, http.MethodGet, "/api/compatibility?direct_play=all", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 2 {
		t.Fatalf("direct_play=all: want 2 items, got %d", len(body.Items))
	}
}

func TestHandleCompatibilityRejectsUnknownProfile(t *testing.T) {
	_, h, _, _ := newTestServer(t, true)

	w := doReq(h, http.MethodGet, "/api/compatibility?client_profile=totally_made_up", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for unknown profile, got %d (%s)", w.Code, w.Body.String())
	}

	w = doReq(h, http.MethodGet, "/api/compatibility/stats?client_profile=totally_made_up", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("stats: want 400 for unknown profile, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestHandleCompatibilityStats(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)
	ctx := t.Context()

	hevc := "hevc"
	id1 := insertAPIMedia(t, st, &store.MediaFile{Path: "/media/movies/a.mkv", VideoCodec: &hevc})
	if err := st.Media.UpsertCompatibility(ctx, id1, map[string]compatibility.Verdict{
		"nvidia_shield": {
			DirectPlayPredicted: false, RiskScore: 12,
			Reasons:           []compatibility.Reason{{Code: "audio_truehd", Severity: compatibility.Advisory, Message: "x"}},
			RecommendedAction: compatibility.ActionAudioTranscode,
		},
	}); err != nil {
		t.Fatal(err)
	}
	h264 := "h264"
	id2 := insertAPIMedia(t, st, &store.MediaFile{Path: "/media/movies/b.mp4", VideoCodec: &h264})
	if err := st.Media.UpsertCompatibility(ctx, id2, map[string]compatibility.Verdict{
		"nvidia_shield": {DirectPlayPredicted: true, RecommendedAction: compatibility.ActionNone},
	}); err != nil {
		t.Fatal(err)
	}

	w := doReq(h, http.MethodGet, "/api/compatibility/stats?client_profile=nvidia_shield", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var stats struct {
		ClientProfile       string `json:"client_profile"`
		TotalFiles          int64  `json:"total_files"`
		DirectPlayCount     int64  `json:"direct_play_count"`
		TranscodeRiskCount  int64  `json:"transcode_risk_count"`
		SavingsOverlapCount int64  `json:"savings_overlap_count"`
		ByReason            []struct {
			Code      string `json:"code"`
			FileCount int64  `json:"file_count"`
		} `json:"by_reason"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.ClientProfile != "nvidia_shield" || stats.TotalFiles != 2 || stats.DirectPlayCount != 1 || stats.TranscodeRiskCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(stats.ByReason) != 1 || stats.ByReason[0].Code != "audio_truehd" || stats.ByReason[0].FileCount != 1 {
		t.Fatalf("unexpected by_reason: %+v", stats.ByReason)
	}
}

func TestHandleFileDetailIncludesStreamsAndCompatibility(t *testing.T) {
	_, h, st, _ := newTestServer(t, true)
	ctx := t.Context()

	hevc := "hevc"
	id := insertAPIMedia(t, st, &store.MediaFile{Path: "/media/movies/detail.mkv", VideoCodec: &hevc})

	codec := "hevc"
	audioCodec := "dts"
	if err := st.Streams.ReplaceForFile(ctx, id, []store.MediaStream{
		{StreamIndex: 0, CodecType: "video", CodecName: &codec, DispositionDefault: true},
		{StreamIndex: 1, CodecType: "audio", CodecName: &audioCodec},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Media.UpsertCompatibility(ctx, id, map[string]compatibility.Verdict{
		"apple_tv_4k": {
			DirectPlayPredicted: false, RiskScore: 10,
			Reasons:           []compatibility.Reason{{Code: "audio_dts", Severity: compatibility.Advisory, Message: "x"}},
			RecommendedAction: compatibility.ActionAudioTranscode,
		},
		"plex_web": {
			DirectPlayPredicted: false, RiskScore: 45,
			Reasons:           []compatibility.Reason{{Code: "video_codec_hevc", Severity: compatibility.Hard, Message: "y"}},
			RecommendedAction: compatibility.ActionManual,
		},
	}); err != nil {
		t.Fatal(err)
	}

	w := doReq(h, http.MethodGet, "/api/files/"+itoa(id), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var dto mediaFileDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if len(dto.Streams) != 2 {
		t.Fatalf("want 2 streams, got %d: %+v", len(dto.Streams), dto.Streams)
	}
	if len(dto.Compatibility) != 2 {
		t.Fatalf("want 2 compatibility verdicts (2 profiles evaluated), got %d: %+v", len(dto.Compatibility), dto.Compatibility)
	}
	byProfile := map[string]compatibilityDTO{}
	for _, c := range dto.Compatibility {
		byProfile[c.ClientProfile] = c
	}
	if byProfile["plex_web"].RiskScore != 45 {
		t.Fatalf("plex_web risk_score = %d, want 45", byProfile["plex_web"].RiskScore)
	}
}

func itoa(id int64) string {
	b, _ := json.Marshal(id)
	return string(b)
}
