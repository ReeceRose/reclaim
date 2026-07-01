import { Skeleton } from "@/components/ui/skeleton";

export function CandidatesPageSkeleton() {
  return (
    <div className="flex flex-col min-w-0 h-screen overflow-hidden max-sm:h-full">
      <div className="px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]">
        <Skeleton className="h-7 w-48 mb-2" />
        <Skeleton className="h-3 w-52" />
      </div>
    </div>
  );
}
