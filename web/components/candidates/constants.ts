export const CANDIDATES_PAGE_SIZE = 100;

export type CandidateSortKey =
  | "savings_desc"
  | "size_desc"
  | "size_asc"
  | "mtime_desc"
  | "mtime_asc"
  | "codec";

export const CANDIDATE_SORT_OPTIONS: {
  value: CandidateSortKey;
  label: string;
}[] = [
  { value: "savings_desc", label: "Predicted savings" },
  { value: "size_desc", label: "Largest file" },
  { value: "size_asc", label: "Smallest file" },
  { value: "mtime_desc", label: "Newest file" },
  { value: "mtime_asc", label: "Oldest file" },
  { value: "codec", label: "Source codec" },
];

export type CandidateSortColumn = "file" | "codec" | "size" | "savings";

const SORT_COLUMN: Record<CandidateSortKey, CandidateSortColumn> = {
  savings_desc: "savings",
  size_desc: "size",
  size_asc: "size",
  mtime_desc: "file",
  mtime_asc: "file",
  codec: "codec",
};

const COLUMN_DEFAULT_SORT: Record<CandidateSortColumn, CandidateSortKey> = {
  file: "mtime_desc",
  codec: "codec",
  size: "size_desc",
  savings: "savings_desc",
};

const COLUMN_TOGGLE: Partial<
  Record<CandidateSortColumn, readonly [CandidateSortKey, CandidateSortKey]>
> = {
  file: ["mtime_desc", "mtime_asc"],
  size: ["size_desc", "size_asc"],
};

export function candidateSortColumn(
  sort: CandidateSortKey,
): CandidateSortColumn {
  return SORT_COLUMN[sort];
}

export function candidateSortArrow(sort: CandidateSortKey): "↑" | "↓" | null {
  if (sort === "codec") return "↑";
  if (sort.endsWith("_asc")) return "↑";
  if (sort.endsWith("_desc")) return "↓";
  return null;
}

export function toggleCandidateSort(
  sort: CandidateSortKey,
  column: CandidateSortColumn,
): CandidateSortKey {
  const toggle = COLUMN_TOGGLE[column];
  if (candidateSortColumn(sort) === column && toggle) {
    return sort === toggle[0] ? toggle[1] : toggle[0];
  }
  return COLUMN_DEFAULT_SORT[column];
}
