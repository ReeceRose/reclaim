'use client';

import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';

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
    <div className="border border-line rounded-(--radius) p-5 mt-[18px]" style={{ background: 'var(--surface)' }}>
      <div className="text-[0.72rem] uppercase tracking-[0.11em] text-muted-fg font-bold mb-4">Metadata</div>
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <Label className="text-[0.8rem] font-semibold block mb-0.5">TMDB API key</Label>
          {tmdbConfigured ? (
            <p className="text-[0.78rem] text-green font-mono">Configured · TMDB_API_KEY env var</p>
          ) : (
            <p className="text-[0.78rem] text-muted-dim font-mono">Not configured · set <span className="text-muted-fg">TMDB_API_KEY</span> env var</p>
          )}
        </div>
        {tmdbConfigured && (
          <Button
            variant="outline"
            onClick={onRefresh}
            disabled={isRefreshing}
            className="rounded-[11px]"
          >
            {isRefreshing ? 'Refreshing…' : 'Refresh all metadata'}
          </Button>
        )}
      </div>
    </div>
  );
}
