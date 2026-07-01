import { Skeleton } from "@/components/ui/skeleton";

/**
 * GroupedSkeleton is the loading placeholder for the grouped (by-series) media
 * list. `withCheckbox` matches the candidate browser's selectable rows; the
 * library leaves it off.
 */
export function GroupedSkeleton({
  withCheckbox = false,
}: {
  withCheckbox?: boolean;
}) {
  return (
    <div className="bg-surface border border-line rounded-(--radius) overflow-hidden">
      {[0, 1, 2, 3].map((i) => (
        <div
          key={i}
          className="flex items-center gap-[11px] px-4 py-[13px] border-b border-line-soft"
        >
          {withCheckbox && (
            <Skeleton className="w-[17px] h-[17px] rounded-[5px] shrink-0" />
          )}
          <Skeleton className="w-[18px] h-[18px] shrink-0 rounded" />
          <div className="flex-1 min-w-0">
            <Skeleton className="h-4 w-48 mb-1.5" />
            <Skeleton className="h-3 w-64" />
          </div>
          {withCheckbox ? (
            <div className="text-right shrink-0">
              <Skeleton className="h-3 w-20 mb-1 ml-auto" />
              <Skeleton className="h-4 w-16 ml-auto" />
            </div>
          ) : (
            <Skeleton className="h-4 w-20" />
          )}
        </div>
      ))}
    </div>
  );
}
