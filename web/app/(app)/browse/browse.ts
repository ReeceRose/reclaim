export const BROWSE_ROUTES = {
  ROOT: (view?: string) =>
    view && view !== VIEW_MODE.GRID ? `/browse?view=${view}` : "/browse",
  TV_SHOW: (title: string, view?: string) =>
    `/browse/tv?show=${encodeURIComponent(title)}${view && view !== VIEW_MODE.GRID ? `&view=${view}` : ""}`,
  MOVIE: (id: number) => `/browse/file?id=${id}`,
  FILE: (id: number) => `/browse/file?id=${id}`,
};

export const QUERY_PARAMS = {
  TAB: "tab",
  TV_SORT: "tvsort",
  MOVIE_SORT: "msort",
  SEASON_SORT: "ssort",
  VIEW: "view",
} as const;

export const BROWSE_TAB = {
  TV: "tv",
  MOVIES: "movies",
  SEASONS: "seasons",
} as const;

export const VIEW_MODE = {
  GRID: "grid",
  LIST: "list",
} as const;

export const LIBRARY_TYPE = {
  TV: "tv",
  MOVIES: "movies",
} as const;

export const TV_SORT = {
  ALPHA: "alpha",
  SAVINGS: "savings",
  SIZE: "size",
  FILES: "files",
} as const;

export const MOVIE_SORT = {
  ALPHA: "path_asc",
  SIZE: "size_desc",
  RECENT: "mtime_desc",
} as const;

export const SEASON_SORT = {
  SIZE: "size_desc",
  SAVINGS: "savings_desc",
} as const;

export const PAGE_SIZE = 48;
export const EPISODES_PER_PAGE = 100;

// Re-exported from the canonical source so existing `./browse` imports keep
// working while the palette lives in one place.
export { CODEC_BORDER, CODEC_COLORS } from "@/lib/codec";
