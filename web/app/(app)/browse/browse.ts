export const BROWSE_ROUTES = {
  ROOT: (view?: string) => view && view !== VIEW_MODE.GRID ? `/browse?view=${view}` : '/browse',
  TV_SHOW: (title: string, view?: string) =>
    `/browse/tv?show=${encodeURIComponent(title)}${view && view !== VIEW_MODE.GRID ? `&view=${view}` : ''}`,
  MOVIE: (id: number) => `/browse/file?id=${id}`,
  FILE: (id: number) => `/browse/file?id=${id}`,
};

export const QUERY_PARAMS = {
  TAB: 'tab',
  TV_SORT: 'tvsort',
  MOVIE_SORT: 'msort',
  VIEW: 'view',
} as const;

export const VIEW_MODE = {
  GRID: 'grid',
  LIST: 'list',
} as const;

export const LIBRARY_TYPE = {
  TV: 'tv',
  MOVIES: 'movies',
} as const;

export const TV_SORT = {
  ALPHA: 'alpha',
  SAVINGS: 'savings',
  SIZE: 'size',
  FILES: 'files',
} as const;

export const MOVIE_SORT = {
  ALPHA: 'path_asc',
  SIZE: 'size_desc',
  RECENT: 'mtime_desc',
} as const;

export const PAGE_SIZE = 48;
export const EPISODES_PER_PAGE = 100;

export const CODEC_COLORS: Record<string, string> = {
  h264: 'text-gold',
  hevc: 'text-green',
  h265: 'text-green',
  mpeg2: 'text-rose',
  mpeg2video: 'text-rose',
  vc1: 'text-violet',
  av1: 'text-sky',
};

export const CODEC_BORDER: Record<string, string> = {
  h264: 'border-[rgba(241,194,27,.3)] bg-[rgba(241,194,27,.1)]',
  hevc: 'border-green-soft bg-green-soft',
  h265: 'border-green-soft bg-green-soft',
  mpeg2: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  mpeg2video: 'border-[rgba(255,126,182,.3)] bg-[rgba(255,126,182,.1)]',
  vc1: 'border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]',
  av1: 'border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]',
};
