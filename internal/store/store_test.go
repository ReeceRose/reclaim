package store

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrate_idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer s2.Close()

	version, err := s2.Version()
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != 12 {
		t.Fatalf("want version 12, got %d", version)
	}
}

func TestDefaultProfile_seededOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := s1.Profiles.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 {
		t.Fatalf("want 1 default profile, got %d", len(profiles))
	}
	if profiles[0].Name != "Space Saver" || !profiles[0].IsDefault {
		t.Fatalf("unexpected profile: %+v", profiles[0])
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	profiles2, err := s2.Profiles.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles2) != 1 {
		t.Fatalf("second boot: want 1 profile, got %d", len(profiles2))
	}
}

func TestProfiles_defaultIsUnique(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	secondID, err := s.Profiles.Create(ctx, &TranscodeProfile{
		Name:      "Quality",
		CRF:       22,
		Preset:    "slow",
		IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	profiles, err := s.Profiles.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertDefaultProfile(t, profiles, secondID)

	thirdID, err := s.Profiles.Create(ctx, &TranscodeProfile{
		Name:   "Fast",
		CRF:    28,
		Preset: "fast",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Profiles.Update(ctx, &TranscodeProfile{
		ID:        thirdID,
		Name:      "Fast",
		CRF:       28,
		Preset:    "fast",
		IsDefault: true,
	}); err != nil {
		t.Fatal(err)
	}

	profiles, err = s.Profiles.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	assertDefaultProfile(t, profiles, thirdID)
	if !profiles[0].IsDefault {
		t.Fatalf("want default profile first in list, got %+v", profiles[0])
	}
}

func assertDefaultProfile(t *testing.T, profiles []TranscodeProfile, wantID int64) {
	t.Helper()

	var defaults []TranscodeProfile
	for _, p := range profiles {
		if p.IsDefault {
			defaults = append(defaults, p)
		}
	}
	if len(defaults) != 1 {
		t.Fatalf("want exactly 1 default profile, got %d: %+v", len(defaults), profiles)
	}
	if defaults[0].ID != wantID {
		t.Fatalf("want default profile id %d, got %d", wantID, defaults[0].ID)
	}
}

func TestSessionSecret_stableAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sec1 := s1.Settings.SessionSecret()
	if len(sec1) == 0 {
		t.Fatal("expected non-empty session secret")
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	sec2 := s2.Settings.SessionSecret()
	if !bytes.Equal(sec1, sec2) {
		t.Fatalf("secret changed across reopen: %x vs %x", sec1, sec2)
	}
}

func TestSettings_authRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if s.Settings.IsSetupComplete() {
		t.Fatal("expected setup incomplete on fresh db")
	}
	if err := s.Settings.CompleteSetup(ctx, "admin", "secret-pass"); err != nil {
		t.Fatal(err)
	}
	if !s.Settings.IsSetupComplete() {
		t.Fatal("setup should be complete")
	}
	if !s.Settings.ValidateLogin("admin", "secret-pass") {
		t.Fatal("login should succeed")
	}
	if s.Settings.ValidateLogin("admin", "wrong") {
		t.Fatal("wrong password should fail")
	}
	if err := s.Settings.CompleteSetup(ctx, "other", "pass"); !errors.Is(err, ErrSetupAlreadyComplete) {
		t.Fatalf("want ErrSetupAlreadyComplete, got %v", err)
	}
}

func TestSettings_resetAuth(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.Settings.CompleteSetup(ctx, "admin", "pass"); err != nil {
		t.Fatal(err)
	}
	if err := s.Settings.ResetAuth(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Settings.IsSetupComplete() {
		t.Fatal("setup should be incomplete after reset")
	}
	if s.Settings.ValidateLogin("admin", "pass") {
		t.Fatal("login should fail after reset")
	}
}

func TestSettings_changeCredentials(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.Settings.CompleteSetup(ctx, "admin", "old-pass"); err != nil {
		t.Fatal(err)
	}
	if err := s.Settings.ChangeCredentials(ctx, "newadmin", "new-pass"); err != nil {
		t.Fatal(err)
	}
	if s.Settings.ValidateLogin("admin", "old-pass") {
		t.Fatal("old credentials should not work")
	}
	if !s.Settings.ValidateLogin("newadmin", "new-pass") {
		t.Fatal("new credentials should work")
	}
	if err := s.Settings.ChangeCredentials(ctx, "x", "y"); err != nil {
		t.Fatal(err)
	}
}

func TestSettings_defaultClientProfile(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	got, err := s.Settings.DefaultClientProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "apple_tv_4k" {
		t.Fatalf("default_client_profile = %q, want %q (migration default)", got, "apple_tv_4k")
	}

	if err := s.Settings.SetDefaultClientProfile(ctx, "nvidia_shield"); err != nil {
		t.Fatal(err)
	}
	got, err = s.Settings.DefaultClientProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "nvidia_shield" {
		t.Fatalf("default_client_profile after set = %q, want %q", got, "nvidia_shield")
	}
}

func TestStreams_replaceForFileIsFullSwap(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	fileID, err := s.Media.insertTestRow(ctx, "/movies/test.mkv")
	if err != nil {
		t.Fatal(err)
	}

	codecHEVC, codecTrueHD := "hevc", "truehd"
	channels := 8
	extra := `{"pix_fmt":"yuv420p10le"}`
	first := []MediaStream{
		{StreamIndex: 0, CodecType: "video", CodecName: &codecHEVC, DispositionDefault: true, ExtraJSON: &extra},
		{StreamIndex: 1, CodecType: "audio", CodecName: &codecTrueHD, Channels: &channels},
	}
	if err := s.Streams.ReplaceForFile(ctx, fileID, first); err != nil {
		t.Fatal(err)
	}

	got, err := s.Streams.ListForFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("streams len = %d, want 2", len(got))
	}
	if got[0].CodecName == nil || *got[0].CodecName != "hevc" || !got[0].DispositionDefault {
		t.Errorf("stream[0] = %+v", got[0])
	}
	if got[0].ExtraJSON == nil || *got[0].ExtraJSON != extra {
		t.Errorf("stream[0].ExtraJSON = %v, want %q", got[0].ExtraJSON, extra)
	}

	// A second call with a different (smaller) set must fully replace, not append.
	codecH264 := "h264"
	second := []MediaStream{
		{StreamIndex: 0, CodecType: "video", CodecName: &codecH264},
	}
	if err := s.Streams.ReplaceForFile(ctx, fileID, second); err != nil {
		t.Fatal(err)
	}
	got, err = s.Streams.ListForFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("streams len after replace = %d, want 1", len(got))
	}
	if got[0].CodecName == nil || *got[0].CodecName != "h264" {
		t.Errorf("stream[0] after replace = %+v", got[0])
	}
}

func TestStreams_cascadeDeleteOnMediaFileDelete(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	fileID, err := s.Media.insertTestRow(ctx, "/movies/test.mkv")
	if err != nil {
		t.Fatal(err)
	}
	codec := "hevc"
	if err := s.Streams.ReplaceForFile(ctx, fileID, []MediaStream{
		{StreamIndex: 0, CodecType: "video", CodecName: &codec},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.w.ExecContext(ctx, "DELETE FROM media_files WHERE id = ?", fileID); err != nil {
		t.Fatal(err)
	}

	got, err := s.Streams.ListForFile(ctx, fileID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("streams after media_files delete = %d, want 0 (ON DELETE CASCADE)", len(got))
	}
}

func TestMedia_notFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.Media.GetByID(ctx, 9999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestConcurrency_readersAndWriter(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id, err := s.Media.insertTestRow(ctx, "/movies/test.mkv")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 32)
	stop := make(chan struct{})

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					if _, err := s.Media.GetByID(ctx, id); err != nil {
						errCh <- err
						return
					}
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if err := s.Media.touch(ctx, id); err != nil && !isBusy(err) {
					errCh <- err
					return
				}
			}
		}
	}()

	time.Sleep(3 * time.Second)
	close(stop)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if isBusy(err) {
			t.Fatalf("SQLITE_BUSY during concurrency test: %v", err)
		}
		t.Fatalf("unexpected error: %v", err)
	}
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "database is locked") || strings.Contains(s, "SQLITE_BUSY")
}
