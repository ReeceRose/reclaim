import { Button } from "@/components/ui/button";
import { apiErrorMessage } from "@/lib/query-errors";

export function QueryErrorState({
  error,
  onRetry,
  title = "Failed to load",
}: {
  error: unknown;
  onRetry: () => void;
  title?: string;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-20 text-center">
      <div>
        <div className="text-[0.9rem] font-semibold text-text">{title}</div>
        <div className="text-[0.78rem] text-muted-dim mt-1 max-w-[320px]">
          {apiErrorMessage(error)}
        </div>
      </div>
      <Button variant="outline" size="sm" onClick={onRetry}>
        Retry
      </Button>
    </div>
  );
}
