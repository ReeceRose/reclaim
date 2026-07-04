import { Skeleton } from "@/components/ui/skeleton";

export function SettingsSkeleton() {
  return (
    <div className="px-4 py-6 w-full pb-14 sm:px-7 sm:py-7">
      <div className="grid grid-cols-2 gap-5 mb-5 max-sm:grid-cols-1">
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
                <Skeleton className="h-9 w-full rounded-xl" />
              </div>
            ))}
            <Skeleton className="h-9 w-28 rounded-xl" />
          </div>
        ))}
      </div>
      <div
        className="border border-line rounded-(--radius) p-5"
        style={{ background: "var(--surface)" }}
      >
        <div className="flex items-center mb-4">
          <Skeleton className="h-3 w-36" />
          <Skeleton className="ml-auto h-8 w-28 rounded-xl" />
        </div>
        {[0, 1].map((i) => (
          <div
            key={i}
            className="flex items-center gap-3.5 border border-line rounded-xl px-4 py-3.5 mb-2.5"
          >
            <div className="flex-1 min-w-0">
              <Skeleton className="h-4 w-32 mb-1.5" />
              <Skeleton className="h-3 w-48" />
            </div>
            <Skeleton className="h-7 w-12 rounded-xl" />
            <Skeleton className="h-7 w-14 rounded-xl" />
          </div>
        ))}
      </div>
    </div>
  );
}
