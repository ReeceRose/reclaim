"use client";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";

export function MetadataPanel({
  tmdbConfigured,
  onRefresh,
  isRefreshing,
}: {
  tmdbConfigured: boolean;
  onRefresh: () => void;
  isRefreshing: boolean;
}) {
  return (
    <div
      className="border border-line rounded-(--radius) p-5 mt-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
        Metadata
      </div>
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <Label className="text-xs font-semibold block mb-0.5">
            TMDB API key
          </Label>
          {tmdbConfigured ? (
            <p className="text-xs text-green font-mono">
              Configured · TMDB_API_KEY env var
            </p>
          ) : (
            <p className="text-xs text-muted-dim font-mono">
              Not configured · set{" "}
              <span className="text-muted-fg">TMDB_API_KEY</span> env var
            </p>
          )}
        </div>
        {tmdbConfigured && (
          <Button
            variant="outline"
            onClick={onRefresh}
            disabled={isRefreshing}
            className="rounded-xl"
          >
            {isRefreshing ? "Refreshing…" : "Refresh all metadata"}
          </Button>
        )}
      </div>
    </div>
  );
}
