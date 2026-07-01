# Direct-play compatibility

Reclaim predicts whether a file will **direct-play** on a chosen client profile
(playback device or app) or force Plex, Jellyfin, or Emby to transcode. Predictions
are based entirely on `ffprobe` metadata — there is no Plex/Jellyfin API
integration and no playback telemetry.

Everything in the UI is labeled **predicted**. Actual direct-play depends on
things Reclaim cannot see: media server version, client app build, user settings
(audio passthrough, custom server-side XML profiles), and downstream AVR/receiver
capability.

---

## Two lenses

| Lens | Question | Primary score |
|------|----------|---------------|
| **Savings** (Candidates) | How much space would HEVC reclaim? | `predicted_savings_bytes` |
| **Direct play** (Compatibility) | Will this direct-play on my client? | `compatibility_risk_score` |

The compatibility list **includes already-HEVC files** — unlike Candidates, which
excludes them. Many compatibility issues (DTS audio, MKV container, PGS subtitles)
apply to HEVC rips too.

---

## Client profiles (v1)

Four built-in profiles ship as static rules in `internal/compatibility/profile.go`.
Pick one in **Settings → Compatibility** or on the **Direct play** page. The default
is **Apple TV 4K**.

| ID | Name | Models |
|----|------|--------|
| `apple_tv_4k` | Apple TV 4K | Modern Apple TV hardware via Plex/Jellyfin. HEVC 10-bit HDR supported; MKV and lossless audio depend on server client profile. |
| `nvidia_shield` | NVIDIA Shield | Shield TV / Shield TV Pro. Broadest video decode (incl. MPEG-2). TrueHD/DTS-HD pass through over HDMI when AVR supports it. No AV1. |
| `plex_web` | Plex Web / Browser | Plex's default **browser** client profile — H.264 + AAC in MP4, 8-bit SDR only. Not the browser's raw decode capability. |
| `generic_hevc` | Generic HEVC client | Synthetic baseline for Kodi / Jellyfin Media Player (desktop). Loose reference — not device-accurate for Android TV, Roku, etc. |

Custom profiles are not supported in v1.

---

## Risk score and reasons

Each file × profile gets:

- **`direct_play_predicted`** — `true` when no reasons were found
- **`risk_score`** — 0–100; higher = more likely to transcode
- **`reasons`** — ordered list with `code`, `severity` (`hard` | `advisory`), optional `stream` index, and a human `message`
- **`recommended_action`** — suggested fix path (`reencode_hevc`, `remux`, `audio_transcode`, `manual`, `none`)

**Hard** reasons model stable hardware/software ceilings (unsupported video codec,
PGS subtitles, SDR-only client + HDR metadata).

**Advisory** reasons depend on server profile or AVR passthrough (MKV on Apple TV,
DTS-HD on Shield). They contribute less to `risk_score` and use hedged copy.

### Common reason codes

| Code | Typical cause |
|------|----------------|
| `video_codec_*` | Video codec not in profile allowlist (e.g. `video_codec_mpeg2video`) |
| `hevc_10bit` | HEVC bit depth above profile cap |
| `container_mkv` | Container advisory or hard deny |
| `audio_dts-hd`, `audio_truehd`, … | Audio codec advisory or hard deny |
| `audio_channels_exceeded` | More channels than profile cap |
| `subtitle_pgs` | Image/bitmap subtitles (forces burn-in transcode) |
| `hdr_hdr10` | HDR10 (PQ / `smpte2084`) on SDR-only profile |
| `hdr_hlg` | HLG (`arib-std-b67`) on SDR-only profile |
| `hdr_dolby_vision` | Dolby Vision metadata on SDR-only profile |

Reason codes are generated dynamically — filter by exact code in the API/UI.

---

## HDR (Phase 1.5)

HDR detection uses the primary video stream's `color_transfer` and Dolby Vision
side data from `ffprobe`:

- **HDR10** — `color_transfer = smpte2084`
- **HLG** — `color_transfer = arib-std-b67`
- **Dolby Vision** — Dolby Vision configuration record present

Only **SDR-only profiles** flag HDR today (`plex_web`). HDR-capable profiles
(Apple TV 4K, Shield, generic HEVC) do not add HDR reasons. There is no
automated HDR→SDR fix in v1 — `recommended_action` is `manual`.

---

## Backfill after upgrade

Compatibility data requires extended probe fields (`pixel_format`, `media_streams`,
per-profile verdicts). A normal incremental scan **skips** unchanged files, so
existing libraries need a **full rescan** after upgrading.

On boot, the backfill coordinator detects missing compatibility rows and
auto-starts `POST /api/scan/full` (trigger `backfill`). Progress appears on the
**Direct play** page via the same WebSocket `scan_progress` events as manual scans.
Use `files_processed` (not `files_scanned`) for progress on full rescans.

---

## Limitations

- **No ground truth** — rules are a snapshot of client hardware ceilings, not
  your exact Plex/Jellyfin server XML profiles.
- **Single primary audio stream** — evaluation uses the first audio stream in
  probe order (same as denormalized `audio_codec` on list views). All subtitle
  streams are checked for PGS.
- **Queueing** — Phase 2 (shipped): files whose `recommended_action` is
  `reencode_hevc` can be queued directly from the **Direct play** list (row
  checkboxes) or a file's **Predicted playback** section, through the same
  `libx265` worker path as the Candidate browser. The server validates the
  file's stored verdict for the selected client profile before accepting —
  see `POST /api/jobs` in [`API.md`](API.md#post-apijobs). `remux` and
  `audio_transcode` remain read-only pending Phase 3 (new worker job types).
- **Grouped TV view** — flat list in v1; series grouping deferred to v1.1.

---

## API reference

See [`docs/API.md`](API.md#direct-play-compatibility) for endpoints:
`GET /api/compatibility`, `/stats`, `/profiles`, and file-detail `compatibility[]`.

Implementation plan and sourcing notes: [`docs/COMPATIBILITY PLAN.md`](COMPATIBILITY%20PLAN.md).
