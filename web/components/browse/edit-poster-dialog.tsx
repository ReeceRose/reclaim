"use client";

import Image from "next/image";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { api, type MetadataSearchResult, tmdbImageURL } from "@/lib/api";
import { cn } from "@/lib/utils";

export function EditPosterDialog({
  open,
  onOpenChange,
  showTitle,
  currentPosterPath,
  currentBackdropPath,
  onSaved,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  showTitle: string;
  currentPosterPath?: string | null;
  currentBackdropPath?: string | null;
  onSaved: () => void;
}) {
  const [query, setQuery] = useState(showTitle);
  const [results, setResults] = useState<MetadataSearchResult[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);
  const [saving, setSaving] = useState(false);
  const [customURL, setCustomURL] = useState("");
  const [useCustom, setUseCustom] = useState(false);

  async function handleSearch() {
    if (!query.trim()) return;
    setSearching(true);
    try {
      const res = await api.searchMetadata(query, "tv");
      setResults(res.results);
      setSelected(null);
    } finally {
      setSearching(false);
    }
  }

  async function handleSave() {
    const posterURL = useCustom
      ? customURL.trim() || null
      : selected
        ? selected.replace("/w185/", "/w500/")
        : null;
    if (!posterURL) return;
    setSaving(true);
    try {
      await api.overrideMetadata(
        showTitle,
        "tv",
        posterURL,
        currentBackdropPath ?? null,
      );
      onSaved();
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  }

  const currentPosterURL = tmdbImageURL(currentPosterPath, "w185");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>Edit Poster</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {currentPosterURL && !useCustom && selected === null && (
            <div className="flex items-center gap-3 pb-3 border-b border-line">
              <Image
                src={currentPosterURL}
                alt="current poster"
                width={48}
                height={72}
                className="w-12 h-auto rounded-md shrink-0"
              />
              <span className="text-sm text-muted-fg">Current poster</span>
            </div>
          )}

          <div className="flex gap-2">
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void handleSearch();
              }}
              placeholder="Search TMDB…"
              className="flex-1"
            />
            <Button
              variant="outline"
              onClick={() => void handleSearch()}
              disabled={searching}
            >
              {searching ? "Searching…" : "Search"}
            </Button>
          </div>

          {results.length > 0 && !useCustom && (
            <div className="grid grid-cols-4 gap-2 max-h-72 overflow-y-auto">
              {results.map((r) => (
                <button
                  type="button"
                  key={r.tmdb_id}
                  onClick={() => setSelected(r.poster_url)}
                  className={cn(
                    "rounded-md overflow-hidden border-2 transition-colors cursor-pointer",
                    selected === r.poster_url
                      ? "border-brand"
                      : "border-transparent hover:border-line",
                  )}
                >
                  {r.poster_url ? (
                    <div className="relative w-full aspect-2/3">
                      <Image
                        src={r.poster_url}
                        alt={r.title}
                        fill
                        sizes="120px"
                        className="object-cover"
                      />
                    </div>
                  ) : (
                    <div className="w-full aspect-2/3 bg-surface-3 flex items-center justify-center text-xs text-muted-dim p-1 text-center">
                      {r.title}
                    </div>
                  )}
                  <div className="text-xs text-muted-dim px-1 py-0.5 truncate">
                    {r.title} {r.year ? `(${r.year})` : ""}
                  </div>
                </button>
              ))}
            </div>
          )}

          <div>
            <button
              type="button"
              className="text-xs text-muted-fg hover:text-text underline cursor-pointer"
              onClick={() => {
                setUseCustom(!useCustom);
                setSelected(null);
              }}
            >
              {useCustom ? "Search TMDB instead" : "Use a custom URL instead"}
            </button>
          </div>

          {useCustom && (
            <Input
              value={customURL}
              onChange={(e) => setCustomURL(e.target.value)}
              placeholder="https://… (full poster image URL)"
            />
          )}
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            onClick={() => void handleSave()}
            disabled={saving || (!selected && !(useCustom && customURL.trim()))}
          >
            {saving ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
