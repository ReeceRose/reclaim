package api

import (
	"context"
	"net/http"
	"testing"

	"reclaim/internal/media"
	"reclaim/internal/store"
)

func TestProfilesValidateAgainstTargetEncoder(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)

	cases := []struct {
		name      string
		body      map[string]any
		want      int
		wantCodec string
	}{
		{"av1", map[string]any{"name": "Small", "codec": "av1", "crf": 30, "preset": "6"}, http.StatusCreated, "av1"},
		{"av1 rejects an x265 preset", map[string]any{"name": "Bad", "codec": "av1", "crf": 30, "preset": "medium"}, http.StatusBadRequest, ""},
		{"av1 allows crf above the hevc range", map[string]any{"name": "Tiny", "codec": "av1", "crf": 60, "preset": "8"}, http.StatusCreated, "av1"},
		{"hevc rejects an av1 crf", map[string]any{"name": "Bad", "codec": "hevc", "crf": 60, "preset": "medium"}, http.StatusBadRequest, ""},
		{"hevc rejects an av1 preset", map[string]any{"name": "Bad", "codec": "hevc", "crf": 26, "preset": "6"}, http.StatusBadRequest, ""},
		{"omitted codec is hevc", map[string]any{"name": "Legacy", "crf": 26, "preset": "Medium"}, http.StatusCreated, "hevc"},
		{"unsupported codec", map[string]any{"name": "Bad", "codec": "vp9", "crf": 30, "preset": "6"}, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doReq(h, http.MethodPost, "/api/profiles", tc.body, cookie)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tc.want, w.Body.String())
			}
			if tc.wantCodec != "" {
				if got := decodeBody(t, w)["codec"]; got != tc.wantCodec {
					t.Errorf("codec = %v, want %s", got, tc.wantCodec)
				}
			}
		})
	}
}

func TestEncodersEndpointGatesUnavailableEncoders(t *testing.T) {
	srv, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	srv.encoders = map[media.TargetCodec]bool{media.TargetHEVC: true}

	w := doReq(h, http.MethodGet, "/api/encoders", nil, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("encoders: %d %s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["savings_target_codec"] != "hevc" {
		t.Errorf("savings_target_codec = %v, want hevc", body["savings_target_codec"])
	}
	avail := map[string]bool{}
	for _, it := range body["items"].([]any) {
		e := it.(map[string]any)
		avail[e["codec"].(string)] = e["available"].(bool)
	}
	if !avail["hevc"] || avail["av1"] {
		t.Fatalf("availability = %v, want hevc only", avail)
	}

	w = doReq(h, http.MethodPost, "/api/profiles", map[string]any{
		"name": "Small", "codec": "av1", "crf": 30, "preset": "6",
	}, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("av1 profile without libsvtav1: want 400, got %d", w.Code)
	}

	ctx := context.Background()
	profileID, err := st.Profiles.Create(ctx, &store.TranscodeProfile{Name: "Stale", Codec: "av1", CRF: 30, Preset: "6"})
	if err != nil {
		t.Fatal(err)
	}
	codec := "h264"
	fileID, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: "/media/movies/a.mkv", LibraryType: "movie", SizeBytes: 5000,
		Mtime: 1, Fingerprint: "fpa", VideoCodec: &codec, Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	w = doReq(h, http.MethodPost, "/api/jobs", map[string]any{
		"file_ids": []int64{fileID}, "profile_id": profileID,
	}, cookie)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("queueing on an unavailable encoder: want 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDefaultAV1ProfileRepricesAndQueuesAV1(t *testing.T) {
	_, h, st, _ := newTestServer(t, false)
	cookie := completeSetup(t, st)
	ctx := context.Background()

	codec := "h264"
	duration := 3600.0
	fileID, err := st.Media.Insert(ctx, &store.MediaFile{
		Path: "/media/movies/a.mkv", LibraryType: "movie", SizeBytes: 10_000,
		Mtime: 1, Fingerprint: "fpa", VideoCodec: &codec, PredictedSavingsBytes: 4_000,
		Status: "active", DurationSeconds: &duration,
	})
	if err != nil {
		t.Fatal(err)
	}

	w := doReq(h, http.MethodPost, "/api/profiles", map[string]any{
		"name": "AV1", "codec": "av1", "crf": 30, "preset": "6", "is_default": true,
	}, cookie)
	if w.Code != http.StatusCreated {
		t.Fatalf("create default av1 profile: %d %s", w.Code, w.Body.String())
	}

	body := decodeBody(t, doReq(h, http.MethodGet, "/api/stats", nil, cookie))
	if body["savings_target_codec"] != "av1" {
		t.Errorf("savings_target_codec = %v, want av1", body["savings_target_codec"])
	}
	if got := body["total_recoverable_bytes"].(float64); got != 5_200 {
		t.Errorf("total_recoverable_bytes = %v, want 5200 (h264 priced against AV1)", got)
	}

	w = doReq(h, http.MethodPost, "/api/jobs", map[string]any{"file_ids": []int64{fileID}}, cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("queue: %d %s", w.Code, w.Body.String())
	}

	items := decodeBody(t, doReq(h, http.MethodGet, "/api/jobs?status=queued", nil, cookie))["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("queued jobs = %d, want 1", len(items))
	}
	job := items[0].(map[string]any)
	if job["encode_codec"] != "av1" || job["encode_preset"] != "6" {
		t.Errorf("job snapshot = %v/%v, want av1/6", job["encode_codec"], job["encode_preset"])
	}
	if got := job["predicted_savings_bytes"].(float64); got != 5_200 {
		t.Errorf("predicted_savings_bytes = %v, want 5200", got)
	}
	if src := job["estimate_source"]; src != "seed" {
		t.Errorf("estimate_source = %v, want the AV1 seed", src)
	}
}
