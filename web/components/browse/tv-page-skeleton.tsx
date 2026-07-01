import { Skeleton } from "@/components/ui/skeleton";

export function TvPageSkeleton() {
  return (
    <div className="flex flex-col min-w-0">
      <div className="px-4 py-4 border-b border-line sm:px-7">
        <Skeleton className="h-3 w-16 mb-4" />
        <Skeleton className="h-8 w-56 mb-3" />
        <Skeleton className="h-3 w-44 mb-5" />
        <Skeleton className="h-1 w-full rounded-full" />
      </div>
    </div>
  );
}
