import Link from "next/link";
import { ColumnCells } from "@/components/table/column-cells";
import type { MediaFile } from "@/lib/api";
import type { ColumnDef } from "@/lib/table-columns";

export const MOVIE_ROW_CLASS = "flex items-center gap-2 px-4";

export function MovieRow<S extends string>({
  file,
  columns,
  href,
}: {
  file: MediaFile;
  columns: readonly ColumnDef<MediaFile, S>[];
  href: string;
}) {
  return (
    <Link
      href={href}
      className={`${MOVIE_ROW_CLASS} py-2.5 border-b border-line-soft last:border-b-0 cursor-pointer hover:bg-surface-2 transition-colors`}
    >
      <ColumnCells columns={columns} item={file} />
    </Link>
  );
}
