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
  ratio_source: 'seed' | 'learned';
  learned_sample_count?: number;
}

export interface ResolutionStat {
  band: string;
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
}

export interface KeysetCursor {
  after_savings: number;
  after_id: number;
}

export interface CandidatesPage {
  items: MediaFile[];
  next_cursor?: KeysetCursor;
}

export interface Episode extends MediaFile {
  season: number;
  episode: number | null;
}

export interface SeasonGroup {
  season: number;
  candidate_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  episodes: Episode[];
}

export interface SeriesGroup {
  title: string;
  library_type: string;
  candidate_count: number;
  season_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
  seasons: SeasonGroup[];
}

export interface GroupedCandidates {
  series: SeriesGroup[];
  movies: MediaFile[];
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
}

export interface DryRunResult {
  file_count: number;
  total_bytes: number;
  predicted_savings_bytes: number;
}

export interface Settings {
  encode_window_start: string;
  encode_window_end: string;
  scan_interval: string;
  scan_anchor: string;
  probe_concurrency: number;
  movies_path: string;
  tv_path: string;
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
  type: 'job_completed' | 'job_failed' | 'job_cancelled' | 'scan_completed' | 'orphan_restored';
  severity: 'info' | 'warn' | 'error';
  message: string;
  metadata: Record<string, unknown> | null;
  created_at: number;
}

// ---------------------------------------------------------------------------
// Query params
// ---------------------------------------------------------------------------

export interface CandidateFilters {
  sort?: string;
  library_type?: string;
  video_codec?: string;
  resolution_band?: string;
  search?: string;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
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
  candidates: (filters: CandidateFilters & { limit?: number; offset?: number; after_savings?: number; after_id?: number }) =>
    request<CandidatesPage>("GET", `/api/candidates${buildQuery(filters)}`),
  groupedCandidates: (filters: CandidateFilters) =>
    request<GroupedCandidates>("GET", `/api/candidates/grouped${buildQuery(filters)}`),
  dryRun: (params: { ids?: string } & CandidateFilters) =>
    request<DryRunResult>("GET", `/api/dry-run${buildQuery(params)}`),

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
  jobs: (status?: string) =>
    request<{ items: Job[] }>("GET", `/api/jobs${buildQuery({ status })}`),
  cancelJob: (id: number) =>
    request<{ job_id: number; status: string }>("POST", `/api/jobs/${id}/cancel`),
  forceJob: (id: number) =>
    request<{ job_id: number; forced: boolean }>("POST", `/api/jobs/${id}/force`),

  // Events
  events: (params?: { after_id?: number; limit?: number; severity?: string; type?: string }) =>
    request<{ items: AppEvent[]; next_cursor?: number }>("GET", `/api/events${buildQuery(params ?? {})}`),

  // Settings
  settings: () => request<Settings>("GET", "/api/settings"),
  updateSettings: (s: Partial<Pick<Settings, "encode_window_start" | "encode_window_end" | "scan_interval" | "scan_anchor" | "probe_concurrency">>) =>
    request<Settings>("PUT", "/api/settings", s),
};

/** wsURL builds the WebSocket URL for live progress, honoring the API base. */
export function wsURL(): string {
  if (typeof window === "undefined") return "";
  const base = BASE || window.location.origin;
  const u = new URL("/api/ws", base);
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  return u.toString();
}
