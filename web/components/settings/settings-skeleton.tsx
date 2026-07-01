import { Skeleton } from "@/components/ui/skeleton";

export function SettingsSkeleton() {
  return (
    <div className="px-4 py-[22px] w-full pb-14 sm:px-7 sm:py-[26px]">
      <div className="grid grid-cols-2 gap-[18px] mb-[18px] max-sm:grid-cols-1">
        {[0, 1].map((i) => (
          <div
            key={i}
            className="border border-line rounded-(--radius) p-5"
            style={{ background: "var(--surface)" }}
          >
            <Skeleton className="h-3 w-24 mb-5" />
            {[0, 1, 2].map((j) => (
              <div key={j} className="mb-4">
                <Skeleton className="h-3 w-28 mb-2" />
                <Skeleton className="h-9 w-full rounded-[10px]" />
              </div>
            ))}
            <Skeleton className="h-9 w-28 rounded-[11px]" />
          </div>
        ))}
      </div>
      <div
        className="border border-line rounded-(--radius) p-5"
        style={{ background: "var(--surface)" }}
      >
        <div className="flex items-center mb-4">
          <Skeleton className="h-3 w-36" />
          <Skeleton className="ml-auto h-8 w-28 rounded-[11px]" />
        </div>
        {[0, 1].map((i) => (
          <div
            key={i}
            className="flex items-center gap-3.5 border border-line rounded-[12px] px-4 py-[14px] mb-2.5"
          >
            <div className="flex-1 min-w-0">
              <Skeleton className="h-4 w-32 mb-1.5" />
              <Skeleton className="h-3 w-48" />
            </div>
            <Skeleton className="h-7 w-12 rounded-[11px]" />
            <Skeleton className="h-7 w-14 rounded-[11px]" />
          </div>
        ))}
      </div>
    </div>
  );
}
