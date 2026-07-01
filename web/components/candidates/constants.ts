export const CANDIDATES_PAGE_SIZE = 100;

export type CandidateSortKey =
  | "savings_desc"
  | "size_desc"
  | "mtime_desc"
  | "mtime_asc"
  | "codec";

export const CANDIDATE_SORT_OPTIONS: {
  value: CandidateSortKey;
  label: string;
}[] = [
  { value: "savings_desc", label: "Predicted savings" },
  { value: "size_desc", label: "Largest file" },
  { value: "mtime_desc", label: "Newest file" },
  { value: "mtime_asc", label: "Oldest file" },
  { value: "codec", label: "Source codec" },
];
