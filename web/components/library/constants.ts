export type LibrarySortKey =
  | "path_asc"
  | "size_desc"
  | "size_asc"
  | "mtime_desc"
  | "mtime_asc"
  | "codec"
  | "resolution";

export const LIBRARY_SORT_OPTIONS: {
  value: LibrarySortKey;
  label: string;
}[] = [
  { value: "path_asc", label: "Path" },
  { value: "size_desc", label: "Largest file" },
  { value: "size_asc", label: "Smallest file" },
  { value: "mtime_desc", label: "Recently modified" },
  { value: "mtime_asc", label: "Oldest modified" },
  { value: "codec", label: "Codec" },
  { value: "resolution", label: "Resolution" },
];

export type LibrarySortColumn = "file" | "codec" | "res" | "size";

const SORT_COLUMN: Record<LibrarySortKey, LibrarySortColumn> = {
  path_asc: "file",
  mtime_desc: "file",
  mtime_asc: "file",
  size_desc: "size",
  size_asc: "size",
  codec: "codec",
  resolution: "res",
};

const COLUMN_DEFAULT_SORT: Record<LibrarySortColumn, LibrarySortKey> = {
  file: "mtime_desc",
  codec: "codec",
  res: "resolution",
  size: "size_desc",
};

const COLUMN_TOGGLE: Partial<
  Record<LibrarySortColumn, readonly [LibrarySortKey, LibrarySortKey]>
> = {
  file: ["mtime_desc", "mtime_asc"],
  size: ["size_desc", "size_asc"],
};

export function librarySortColumn(sort: LibrarySortKey): LibrarySortColumn {
  return SORT_COLUMN[sort];
}

export function librarySortArrow(sort: LibrarySortKey): "↑" | "↓" | null {
  if (sort === "codec" || sort === "path_asc" || sort.endsWith("_asc")) {
    return "↑";
  }
  if (sort.endsWith("_desc") || sort === "resolution") return "↓";
  return null;
}

export function toggleLibrarySort(
  sort: LibrarySortKey,
  column: LibrarySortColumn,
): LibrarySortKey {
  const toggle = COLUMN_TOGGLE[column];
  if (librarySortColumn(sort) === column) {
    if (toggle) {
      if (sort === toggle[0]) return toggle[1];
      if (sort === toggle[1]) return toggle[0];
      if (column === "file" && sort === "path_asc") return "mtime_desc";
    }
    return COLUMN_DEFAULT_SORT[column];
  }
  return COLUMN_DEFAULT_SORT[column];
}
