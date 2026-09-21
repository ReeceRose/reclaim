export const QUEUE_TAB = {
  QUEUED: "queued",
  HISTORY: "history",
} as const;

export const QUEUE_QUERY_PARAMS = {
  TAB: "tab",
  PAGE: "page",
  SEARCH: "q",
  LIBRARY: "library",
  CODEC: "codec",
  PROFILE: "profile",
  FORCED: "forced",
} as const;

export const QUEUE_PAGE_SIZE = 25;
