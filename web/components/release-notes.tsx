"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDownIcon, ExternalLinkIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Markdown } from "@/components/ui/markdown";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { api, type ReleaseNote } from "@/lib/api";

/**
 * The changelog is compiled into the binary, so it only changes on restart.
 * `whats_new` is the one part that moves — and only in response to this panel
 * acknowledging it — so the query is refetched on ack rather than on a timer.
 */
export function useReleases() {
  return useQuery({
    queryKey: ["releases"],
    queryFn: api.releases,
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  });
}

function ReleaseEntry({
  release,
  expanded,
  onToggle,
}: {
  release: ReleaseNote;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="border-b border-line-soft last:border-0">
      <Button
        variant="ghost"
        onClick={onToggle}
        aria-expanded={expanded}
        className="h-auto w-full justify-start gap-2.5 rounded-none px-6 py-3.5 font-normal"
      >
        <ChevronDownIcon
          aria-hidden
          className={`h-3.5 w-3.5 shrink-0 text-muted-dim transition-transform duration-130 ${
            expanded ? "" : "-rotate-90"
          }`}
        />
        <span className="font-semibold text-sm text-text">{release.tag}</span>
        {release.current && (
          <Badge className="bg-brand-soft px-2 py-px font-bold text-brand">
            Installed
          </Badge>
        )}
        <span className="ml-auto text-xs font-normal text-muted-dim">
          {release.date}
        </span>
      </Button>

      {expanded && (
        <div className="px-6 pb-5">
          <Markdown source={release.body} />
          <a
            href={release.url}
            target="_blank"
            rel="noreferrer"
            className="mt-4 inline-flex items-center gap-1.5 text-xs text-muted-dim transition-colors hover:text-brand"
          >
            View on GitHub
            <ExternalLinkIcon aria-hidden className="h-3 w-3" />
          </a>
        </div>
      )}
    </div>
  );
}

function ReleaseListSkeleton() {
  return (
    <div className="space-y-5 px-6 py-4">
      {["a", "b", "c"].map((id) => (
        <div key={id} className="space-y-2">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-4/5" />
        </div>
      ))}
    </div>
  );
}

export function ReleaseNotesPanel({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const qc = useQueryClient();
  const { data, isLoading, isError } = useReleases();
  const [expanded, setExpanded] = useState<string[]>([]);
  const acked = useRef(false);
  const primed = useRef(false);

  const ack = useMutation({
    mutationFn: () => api.markReleaseSeen(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["releases"] }),
  });

  // Reading the notes is the acknowledgement — there is no separate dismiss.
  // The ref, not `whats_new`, is the guard: the flag stays true until the
  // invalidated query comes back, which would otherwise post twice.
  const { mutate: ackMutate } = ack;
  useEffect(() => {
    if (!open || !data?.whats_new || acked.current) return;
    acked.current = true;
    ackMutate();
  }, [open, data?.whats_new, ackMutate]);

  // Open on the running version, falling back to the newest entry when this
  // build predates the changelog or is a dev build. Only ever primed once, so
  // collapsing every entry by hand does not spring them back open.
  useEffect(() => {
    if (!open || !data?.releases.length || primed.current) return;
    primed.current = true;
    const current = data.releases.find((r) => r.current) ?? data.releases[0];
    setExpanded([current.tag]);
  }, [open, data]);

  const releases = data?.releases ?? [];

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[34rem] sm:max-w-[90vw]">
        <SheetHeader className="pr-12">
          <SheetTitle>Release notes</SheetTitle>
          <p className="text-xs text-muted-dim">
            Running {data?.current_version ?? "…"}
          </p>
        </SheetHeader>

        <div className="flex-1 overflow-y-auto">
          {isLoading ? (
            <ReleaseListSkeleton />
          ) : isError || releases.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center gap-2 py-20 text-muted-dim">
              <p className="text-sm">No release notes in this build</p>
              {data?.repo_url && (
                <a
                  href={`${data.repo_url}/releases`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1.5 text-xs text-brand hover:underline"
                >
                  Browse releases on GitHub
                  <ExternalLinkIcon aria-hidden className="h-3 w-3" />
                </a>
              )}
            </div>
          ) : (
            releases.map((release) => (
              <ReleaseEntry
                key={release.tag}
                release={release}
                expanded={expanded.includes(release.tag)}
                onToggle={() =>
                  setExpanded((prev) =>
                    prev.includes(release.tag)
                      ? prev.filter((t) => t !== release.tag)
                      : [...prev, release.tag],
                  )
                }
              />
            ))
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}
