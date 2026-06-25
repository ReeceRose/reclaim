'use client';

import { createContext, useCallback, useContext, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, type MediaFile } from '@/lib/api';
import {
  baseName,
  dirName,
  formatBytes,
  formatDuration,
  relativeTime,
  resolutionLabel,
} from '@/lib/format';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';

type FileDetailContextValue = {
  openFile: (id: number, initial?: MediaFile) => void;
  closeFile: () => void;
};

const FileDetailContext = createContext<FileDetailContextValue | null>(null);

export function useFileDetail() {
  const ctx = useContext(FileDetailContext);
  if (!ctx) throw new Error('useFileDetail must be used within FileDetailProvider');
  return ctx;
}

function DetailRow({ label, value, mono }: { label: string; value: React.ReactNode; mono?: boolean }) {
  if (value == null || value === '' || value === '—') return null;
  return (
    <div className="flex items-baseline justify-between gap-4 py-[9px] border-b border-line-soft last:border-b-0">
      <span className="text-[0.78rem] text-muted-fg shrink-0">{label}</span>
      <span className={`text-[0.82rem] text-right ${mono ? 'font-mono' : ''}`}>{value}</span>
    </div>
  );
}

function DetailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-5 last:mb-0">
      <div className="text-[0.68rem] uppercase tracking-[0.11em] text-muted-dim font-bold mb-2">{title}</div>
      <div className="rounded-[11px] border border-line px-3" style={{ background: 'var(--surface-2)' }}>
        {children}
      </div>
    </div>
  );
}

function FileDetailContent({ file }: { file: MediaFile }) {
  const resolution =
    file.width && file.height
      ? `${file.width}×${file.height} (${resolutionLabel(file.width, file.height)})`
      : resolutionLabel(file.width, file.height);

  return (
    <div className="flex-1 overflow-auto px-6 py-5">
      <div className="flex flex-wrap gap-2 mb-5">
        <Badge
          className={`text-[0.7rem] rounded-[7px] font-semibold ${
            file.library_type === 'tv'
              ? 'text-sky border-[rgba(51,177,255,.32)] bg-[rgba(51,177,255,.1)]'
              : 'text-violet border-[rgba(190,149,255,.3)] bg-[rgba(190,149,255,.1)]'
          }`}
        >
          {file.library_type === 'tv' ? 'TV' : 'Movie'}
        </Badge>
        {file.is_already_hevc && (
          <Badge className="text-[0.7rem] rounded-[7px] text-green border-green-soft bg-green-soft">HEVC</Badge>
        )}
        {file.status !== 'active' && (
          <Badge variant="secondary" className="text-[0.7rem] rounded-[7px]">{file.status}</Badge>
        )}
      </div>

      <div className="grid grid-cols-2 gap-3 mb-5">
        <div className="rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--surface-2)' }}>
          <div className="text-[0.7rem] uppercase tracking-wider text-muted-fg">Size</div>
          <div className="text-[1.1rem] font-bold font-mono mt-0.5">{formatBytes(file.size_bytes)}</div>
        </div>
        <div className="rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--surface-2)' }}>
          <div className="text-[0.7rem] uppercase tracking-wider text-muted-fg">Est. savings</div>
          <div className="text-[1.1rem] font-bold font-mono mt-0.5 text-brand">{formatBytes(file.predicted_savings_bytes)}</div>
        </div>
      </div>

      <DetailSection title="Video">
        <DetailRow label="Codec" value={file.video_codec} mono />
        <DetailRow label="Profile" value={file.video_codec_profile} mono />
        <DetailRow label="Resolution" value={resolution} />
        <DetailRow label="Bitrate" value={file.bitrate_kbps != null ? `${file.bitrate_kbps.toLocaleString()} kbps` : null} mono />
        <DetailRow label="Duration" value={formatDuration(file.duration_seconds)} />
      </DetailSection>

      <DetailSection title="Audio">
        <DetailRow label="Codec" value={file.audio_codec} mono />
        <DetailRow
          label="Channels"
          value={
            file.audio_channels != null
              ? file.audio_channels === 1
                ? 'Mono'
                : file.audio_channels === 2
                  ? 'Stereo'
                  : `${file.audio_channels} ch`
              : null
          }
        />
      </DetailSection>

      <DetailSection title="Container">
        <DetailRow label="Format" value={file.container_format} mono />
      </DetailSection>

      <DetailSection title="File">
        <DetailRow label="Modified" value={relativeTime(file.mtime)} />
        <DetailRow label="Last probed" value={relativeTime(file.last_probed_at)} />
        {file.probe_error && (
          <div className="py-[9px] border-b border-line-soft last:border-b-0">
            <div className="text-[0.78rem] text-muted-fg mb-1">Probe error</div>
            <div className="text-[0.78rem] text-red font-mono break-all">{file.probe_error}</div>
          </div>
        )}
      </DetailSection>

      <div className="mt-4 rounded-[11px] border border-line px-3 py-3" style={{ background: 'var(--bg)' }}>
        <div className="text-[0.7rem] uppercase tracking-wider text-muted-dim mb-1">Path</div>
        <div className="text-[0.74rem] font-mono text-muted-fg break-all leading-relaxed">{file.path}</div>
      </div>
    </div>
  );
}

function FileDetailSheet({ fileId, initial, onClose }: { fileId: number; initial: MediaFile | null; onClose: () => void }) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['file', fileId],
    queryFn: () => api.file(fileId),
    initialData: initial ?? undefined,
    staleTime: 30_000,
  });

  const file = data ?? initial;

  return (
    <Sheet open onOpenChange={(open) => !open && onClose()}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle className="text-[1.05rem] font-bold tracking-tight pr-8">
            {file ? baseName(file.path) : <Skeleton className="h-5 w-48" />}
          </SheetTitle>
          {file && (
            <p className="text-[0.76rem] text-muted-dim font-mono truncate">{dirName(file.path)}</p>
          )}
        </SheetHeader>

        {isLoading && !file ? (
          <div className="flex-1 overflow-auto px-6 py-5 space-y-4">
            <Skeleton className="h-16 w-full rounded-[11px]" />
            <Skeleton className="h-24 w-full rounded-[11px]" />
            <Skeleton className="h-32 w-full rounded-[11px]" />
          </div>
        ) : isError && !file ? (
          <div className="flex-1 flex items-center justify-center px-6 text-[0.85rem] text-muted-fg">
            Could not load file details.
          </div>
        ) : file ? (
          <FileDetailContent file={file} />
        ) : null}
      </SheetContent>
    </Sheet>
  );
}

export function FileDetailProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<{ id: number; initial: MediaFile | null } | null>(null);

  const openFile = useCallback((id: number, initial?: MediaFile) => {
    setState({ id, initial: initial ?? null });
  }, []);

  const closeFile = useCallback(() => setState(null), []);

  return (
    <FileDetailContext.Provider value={{ openFile, closeFile }}>
      {children}
      {state && (
        <FileDetailSheet
          fileId={state.id}
          initial={state.initial}
          onClose={closeFile}
        />
      )}
    </FileDetailContext.Provider>
  );
}
