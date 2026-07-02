// Typed client for the Reclaim Go API. Same-origin in production (the Go binary
// serves this static build); proxied to the backend by next.config rewrites in
// dev. All requests send cookies so the session rides along automatically.

const BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method,
    credentials: "include",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return undefined as T;

  let data: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }

  if (!res.ok) {
    const message =
      (data && typeof data === "object" && "error" in data
        ? String((data as { error: unknown }).error)
        : null) ?? `request failed (${res.status})`;
    throw new ApiError(res.status, message);
  }
  return data as T;
}

// ---------------------------------------------------------------------------
// Types (wire shapes mirror internal/api DTOs)
// ---------------------------------------------------------------------------

export interface Session {
  setup_complete: boolean;
  authenticated: boolean;
  username: string | null;
}

export interface CodecStat {
  codec: string;
  file_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  ratio_source: "seed" | "learned";
  learned_sample_count?: number;
}

export interface ResolutionStat {
  band: string;
  file_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
}

export interface LibraryStat {
  library_type: string;
  file_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
}

export interface Stats {
  total_files: number;
  total_bytes: number;
  total_recoverable_bytes: number;
  by_codec: CodecStat[];
  by_resolution: ResolutionStat[];
  by_library: LibraryStat[];
}

export interface MediaFile {
  id: number;
  path: string;
  library_type: string;
  size_bytes: number;
  mtime: number;
  video_codec: string | null;
  video_codec_profile: string | null;
  width: number | null;
  height: number | null;
  duration_seconds: number | null;
  bitrate_kbps: number | null;
  audio_codec: string | null;
  audio_channels: number | null;
  container_format: string | null;
  is_already_hevc: boolean;
  predicted_savings_bytes: number;
  last_probed_at: number | null;
  probe_error: string | null;
  status: string;
  candidate_state: CandidateState;
  poster_path?: string | null;
  backdrop_path?: string | null;
  overview?: string | null;
  tagline?: string | null;
  genres?: string[] | null;
  vote_average?: number | null;
  vote_count?: number | null;
  release_year?: number | null;
  runtime_mins?: number | null;
}

export type CandidateState =
  | "candidate"
  | "already_hevc"
  | "probe_failed"
  | "unknown_codec"
  | "queued"
  | "completed"
  | "missing";

export interface KeysetCursor {
  after_savings: number;
  after_id: number;
}

export interface CandidatesPage {
  items: MediaFile[];
  next_cursor?: KeysetCursor;
  total_count?: number;
}

export interface FilesPage {
  items: MediaFile[];
  total_count?: number;
}

export interface Episode extends MediaFile {
  season: number;
  episode: number | null;
}

export interface SeasonGroup {
  season: number;
  file_count: number;
  candidate_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  episode_ids: number[];
  eligible_ids: number[];
}

export interface SeriesGroup {
  title: string;
  library_type: string;
  file_count: number;
  candidate_count: number;
  season_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  seasons: SeasonGroup[];
}

export interface GroupedCandidates {
  series: SeriesGroup[];
  total_count?: number;
}

export interface LibrarySeasonGroup {
  season: number;
  file_count: number;
  eligible_count: number;
  missing_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  episode_ids: number[];
}

export interface LibrarySeriesGroup {
  title: string;
  library_type: string;
  file_count: number;
  eligible_count: number;
  missing_count: number;
  season_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  seasons: LibrarySeasonGroup[];
  poster_path?: string | null;
  backdrop_path?: string | null;
}

export interface GroupedFiles {
  series: LibrarySeriesGroup[];
  total_count?: number;
}

export interface GroupedSeasonEpisodes {
  episodes: Episode[];
  total_count?: number;
}

export interface Profile {
  id: number;
  name: string;
  crf: number;
  preset: string;
  extra_args: string | null;
  is_default: boolean;
}

export interface Job {
  id: number;
  media_file_id: number;
  profile_id: number;
  status: string;
  queued_at: number;
  started_at: number | null;
  completed_at: number | null;
  original_size_bytes: number;
  output_size_bytes: number | null;
  progress_percent: number;
  output_path: string | null;
  error_message: string | null;
  verification_result: string | null;
  source_path: string | null;
  queue_position: number;
  forced: boolean;
  encode_preset: string | null;
  encode_crf: number | null;
  encode_extra_args: string | null;
  estimated_duration_seconds?: number | null;
  encode_duration_seconds?: number | null;
  estimate_source?: string;
  estimate_sample_count?: number | null;
}

export interface JobsListResult {
  items: Job[];
  total_count?: number;
  queue_total_estimated_seconds?: number;
  queued_count?: number;
}

export interface Settings {
  encode_window_start: string;
  encode_window_end: string;
  scan_interval: string;
  scan_anchor: string;
  probe_concurrency: number;
  movies_path: string;
  tv_path: string;
  tmdb_configured?: boolean;
}

export interface MetadataSearchResult {
  tmdb_id: number;
  title: string;
  year: number;
  poster_url: string;
}

export interface MediaMetadata {
  key: string;
  media_type: string;
  tmdb_id: number | null;
  title: string | null;
  tagline: string | null;
  overview: string | null;
  poster_path: string | null;
  backdrop_path: string | null;
  release_year: number | null;
  runtime_mins: number | null;
  vote_average: number | null;
  vote_count: number | null;
  genres: string[] | null;
  status: string | null;
  network: string | null;
  in_production: boolean | null;
  is_manual: boolean;
  no_match: boolean;
}

export function tmdbImageURL(
  path: string | null | undefined,
  size: string,
): string | null {
  if (!path) return null;
  if (path.startsWith("http")) return path;
  return `https://image.tmdb.org/t/p/${size}${path}`;
}

export interface QueuedItem {
  job_id: number;
  media_file_id: number;
  path: string;
}

export interface SkippedItem {
  media_file_id: number;
  reason: string;
}

export interface CreateJobsResult {
  profile: Profile;
  queued: QueuedItem[];
  skipped: SkippedItem[];
}

export interface VerificationResult {
  duration_match?: boolean;
  duration_delta_seconds?: number;
  playable?: boolean;
  stream_count_match?: boolean;
  resolution_match?: boolean;
  passed?: boolean;
  [k: string]: unknown;
}

export interface AppEvent {
  id: number;
  type:
    | "job_completed"
    | "job_failed"
    | "job_cancelled"
    | "scan_completed"
    | "orphan_restored";
  severity: "info" | "warn" | "error";
  message: string;
  metadata: Record<string, unknown> | null;
  created_at: number;
}

export interface ScanProgress {
  scan_run_id: number;
  kind: "incremental" | "full";
  trigger: string;
  started_at: number;
  files_seen: number;
  files_processed: number;
  files_scanned: number;
  files_added: number;
  files_updated: number;
  files_moved: number;
  files_removed: number;
  errors: number;
}

// ---------------------------------------------------------------------------
// Query params
// ---------------------------------------------------------------------------

export interface CandidateFilters {
  sort?: string;
  library_type?: string;
  video_codec?: string;
  height?: string;
  search?: string;
}

export interface FileFilters extends CandidateFilters {
  status?: string;
  candidate_state?: CandidateState | "";
}

// biome-ignore lint/suspicious/noExplicitAny: params come from heterogeneous filter objects across callers
function buildQuery(params: Record<string, any>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== "") q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

// ---------------------------------------------------------------------------
// Endpoints
// ---------------------------------------------------------------------------

export const api = {
  // Auth
  session: () => request<Session>("GET", "/api/session"),
  setup: (username: string, password: string) =>
    request<{ username: string }>("POST", "/api/setup", { username, password }),
  login: (username: string, password: string) =>
    request<{ username: string }>("POST", "/api/login", { username, password }),
  logout: () => request<void>("POST", "/api/logout"),
  changeCredentials: (username: string, password: string) =>
    request<{ username: string }>("PUT", "/api/settings/credentials", {
      username,
      password,
    }),

  // Read side
  stats: () => request<Stats>("GET", "/api/stats"),
  files: (filters: FileFilters & { limit?: number; offset?: number }) =>
    request<FilesPage>("GET", `/api/files${buildQuery(filters)}`),
  groupedFiles: (filters: FileFilters & { limit?: number; offset?: number }) =>
    request<GroupedFiles>("GET", `/api/files/grouped${buildQuery(filters)}`),
  groupedFileEpisodes: (
    filters: FileFilters & {
      series: string;
      season: number;
      limit?: number;
      offset?: number;
    },
  ) =>
    request<GroupedSeasonEpisodes>(
      "GET",
      `/api/files/grouped/episodes${buildQuery(filters)}`,
    ),
  groupedFileSeasons: (series: string) =>
    request<{ seasons: LibrarySeasonGroup[] }>(
      "GET",
      `/api/files/grouped/seasons${buildQuery({ series })}`,
    ),
  candidates: (
    filters: CandidateFilters & {
      limit?: number;
      offset?: number;
      after_savings?: number;
      after_id?: number;
    },
  ) => request<CandidatesPage>("GET", `/api/candidates${buildQuery(filters)}`),
  groupedCandidates: (
    filters: CandidateFilters & { limit?: number; offset?: number },
  ) =>
    request<GroupedCandidates>(
      "GET",
      `/api/candidates/grouped${buildQuery(filters)}`,
    ),
  groupedSeasonEpisodes: (
    filters: CandidateFilters & {
      series: string;
      season: number;
      limit?: number;
      offset?: number;
    },
  ) =>
    request<GroupedSeasonEpisodes>(
      "GET",
      `/api/candidates/grouped/episodes${buildQuery(filters)}`,
    ),
  file: (id: number) => request<MediaFile>("GET", `/api/files/${id}`),

  // Scanning
  scan: () => request<{ started: boolean; kind: string }>("POST", "/api/scan"),

  // Profiles
  profiles: () => request<{ items: Profile[] }>("GET", "/api/profiles"),
  createProfile: (p: Omit<Profile, "id">) =>
    request<Profile>("POST", "/api/profiles", p),
  updateProfile: (id: number, p: Omit<Profile, "id">) =>
    request<Profile>("PUT", `/api/profiles/${id}`, p),
  deleteProfile: (id: number) => request<void>("DELETE", `/api/profiles/${id}`),

  // Jobs
  createJobs: (fileIds: number[], profileId?: number) =>
    request<CreateJobsResult>("POST", "/api/jobs", {
      file_ids: fileIds,
      profile_id: profileId ?? null,
    }),
  jobs: (params?: { status?: string; limit?: number; offset?: number }) =>
    request<JobsListResult>("GET", `/api/jobs${buildQuery(params ?? {})}`),
  cancelJob: (id: number) =>
    request<{ job_id: number; status: string }>(
      "POST",
      `/api/jobs/${id}/cancel`,
    ),
  forceJob: (id: number) =>
    request<{ job_id: number; forced: boolean }>(
      "POST",
      `/api/jobs/${id}/force`,
    ),
  deleteJob: (id: number) => request<void>("DELETE", `/api/jobs/${id}`),

  // Events
  events: (params?: {
    after_id?: number;
    limit?: number;
    severity?: string;
    type?: string;
  }) =>
    request<{ items: AppEvent[]; next_cursor?: number }>(
      "GET",
      `/api/events${buildQuery(params ?? {})}`,
    ),
  deleteEvent: (id: number) => request<void>("DELETE", `/api/events/${id}`),
  clearEvents: () => request<void>("DELETE", "/api/events"),

  // Settings
  settings: () => request<Settings>("GET", "/api/settings"),
  updateSettings: (
    s: Partial<
      Pick<
        Settings,
        | "encode_window_start"
        | "encode_window_end"
        | "scan_interval"
        | "scan_anchor"
        | "probe_concurrency"
      >
    >,
  ) => request<Settings>("PUT", "/api/settings", s),

  // Metadata
  getMetadata: (key: string) =>
    request<MediaMetadata | null>("GET", `/api/metadata${buildQuery({ key })}`),
  searchMetadata: (query: string, type: "tv" | "movie") =>
    request<{ results: MetadataSearchResult[] }>(
      "GET",
      `/api/metadata/search${buildQuery({ query, type })}`,
    ),
  overrideMetadata: (
    key: string,
    mediaType: string,
    posterUrl: string | null,
    backdropUrl: string | null,
  ) =>
    request<{ status: string }>("PUT", "/api/metadata", {
      key,
      media_type: mediaType,
      poster_url: posterUrl,
      backdrop_url: backdropUrl,
    }),
  refreshMetadata: (key?: string, mediaType?: string) =>
    request<{ status: string }>(
      "POST",
      "/api/metadata/refresh",
      key && mediaType ? { key, media_type: mediaType } : {},
    ),
};

/** wsURL builds the WebSocket URL for live progress, honoring the API base. */
export function wsURL(): string {
  if (typeof window === "undefined") return "";
  // In dev the app is served by the Next dev server (:3000), whose rewrite proxy
  // does NOT forward WebSocket upgrades to the Go backend — the socket would just
  // hang. NEXT_PUBLIC_WS_BASE (set in `make dev`) points the socket straight at
  // the backend. As a fallback, the Next dev port also routes there directly so
  // a bare `pnpm run dev` still gets live updates. Production leaves both unset
  // and uses the same origin the Go binary serves from.
  const wsBase = process.env.NEXT_PUBLIC_WS_BASE;
  if (wsBase) return `${wsBase.replace(/\/+$/, "")}/api/ws`;
  if (window.location.port === "3000") {
    return `ws://${window.location.hostname}:8080/api/ws`;
  }
  const base = BASE || window.location.origin;
  const u = new URL("/api/ws", base);
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  return u.toString();
}
