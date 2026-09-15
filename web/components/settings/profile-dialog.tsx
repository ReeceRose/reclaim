"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  ApiError,
  api,
  type Encoder,
  type Profile,
  type TargetCodec,
} from "@/lib/api";
import { LabelWithHelp } from "./help-tip";

export function DeleteProfileDialog({
  profile,
  onClose,
  onConfirm,
}: {
  profile: Profile | null;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <Dialog open={!!profile} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-sm p-0 overflow-hidden border-line"
        style={{ background: "var(--surface)" }}
        showCloseButton={false}
      >
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-line">
          <DialogTitle className="text-lg font-bold tracking-tight">
            Delete profile
          </DialogTitle>
        </DialogHeader>
        <div className="px-6 py-5">
          <p className="text-sm text-muted-fg">
            Delete{" "}
            <span className="font-semibold text-text">
              &ldquo;{profile?.name}&rdquo;
            </span>
            ? This cannot be undone.
          </p>
        </div>
        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} className="rounded-xl">
            Cancel
          </Button>
          <Button
            onClick={() => {
              onConfirm();
              onClose();
            }}
            className="rounded-xl bg-red hover:bg-red/90 text-white border-0"
          >
            Delete
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function crfHelp(codec: TargetCodec, enc: Encoder | undefined) {
  const range = enc ? `${enc.crf_min}–${enc.crf_max}` : "";
  if (codec === "av1") {
    return (
      <>
        <strong>Constant Rate Factor</strong> — the quality target for SVT-AV1.
        Lower means higher quality and bigger files. Range is {range};{" "}
        <strong>28–35</strong> is typical for visually transparent results.
        AV1&rsquo;s scale is not HEVC&rsquo;s: the same number means a different
        quality on each.
      </>
    );
  }
  return (
    <>
      <strong>Constant Rate Factor</strong> — the quality target for libx265.
      Lower means higher quality and bigger files; higher means more compression
      and smaller files. Range is {range}; <strong>24–28</strong> is the sweet
      spot for visually-lossless HEVC. Each +6 roughly halves the bitrate.
    </>
  );
}

function presetHelp(codec: TargetCodec) {
  if (codec === "av1") {
    return (
      <>
        SVT-AV1 presets run from <strong>0</strong> (slowest, smallest) to{" "}
        <strong>13</strong> (fastest). <strong>6</strong> is a balanced default;{" "}
        <strong>4</strong>–<strong>5</strong> squeeze out a little more, while{" "}
        <strong>8</strong>–<strong>10</strong> are much faster at some cost in
        size.
      </>
    );
  }
  return (
    <>
      Encoder speed vs. compression efficiency. Slower presets squeeze out more
      savings at the same CRF but take longer to encode. <strong>medium</strong>{" "}
      is a balanced default; <strong>slow</strong>/<strong>slower</strong> gain
      a few extra percent, while <strong>fast</strong>/<strong>veryfast</strong>{" "}
      trade file size for shorter encode times.
    </>
  );
}

export function ProfileDialog({
  open,
  onClose,
  initial,
}: {
  open: boolean;
  onClose: () => void;
  initial: Partial<Profile> | null;
}) {
  const qc = useQueryClient();
  const { data: encoderData } = useQuery({
    queryKey: ["encoders"],
    queryFn: api.encoders,
    staleTime: Infinity,
  });
  const encoders = encoderData?.items;
  const [name, setName] = useState(initial?.name ?? "");
  const [codec, setCodec] = useState<TargetCodec>(initial?.codec ?? "hevc");
  const [crf, setCrf] = useState(initial?.crf ?? 26);
  const [preset, setPreset] = useState(initial?.preset ?? "medium");
  const [extra, setExtra] = useState(
    initial?.extra_args ?? "-c:a copy -c:s copy",
  );
  const [loading, setLoading] = useState(false);

  const encoder = encoders?.find((e) => e.codec === codec);

  function handleCodecChange(next: string) {
    const enc = encoders?.find((e) => e.codec === next);
    if (!enc) return;
    setCodec(enc.codec);
    setCrf(enc.default_crf);
    setPreset(enc.default_preset);
  }

  async function handleSave() {
    setLoading(true);
    try {
      const body = {
        name,
        codec,
        crf,
        preset,
        extra_args: extra,
        is_default: initial?.is_default ?? false,
      };
      if (initial?.id) {
        await api.updateProfile(initial.id, body);
        toast.success("Profile updated");
      } else {
        await api.createProfile(body);
        toast.success("Profile created");
      }
      qc.invalidateQueries({ queryKey: ["profiles"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
      qc.invalidateQueries({ queryKey: ["candidates"] });
      onClose();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Save failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        className="max-w-md border-line p-0 overflow-hidden"
        style={{ background: "var(--surface)" }}
      >
        <DialogHeader className="px-6 pt-6 pb-4 border-b border-line">
          <DialogTitle className="text-lg font-bold">
            {initial?.id ? "Edit profile" : "New profile"}
          </DialogTitle>
        </DialogHeader>
        <div className="px-6 py-5 space-y-4">
          <div>
            <Label className="text-xs font-semibold mb-1.5 block">Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <LabelWithHelp
              label="Codec"
              help={
                <>
                  The codec the encode produces. <strong>HEVC</strong> (libx265)
                  plays almost everywhere. <strong>AV1</strong> (SVT-AV1) is
                  typically around a fifth smaller at the same quality, but far
                  fewer players can decode it. Files already in HEVC or AV1 are
                  never re-encoded, whichever you pick. The{" "}
                  <strong>default</strong> profile&rsquo;s codec is what
                  predicted savings across the app are priced against.
                </>
              }
            />
            {encoders ? (
              <Select value={codec} onValueChange={handleCodecChange}>
                <SelectTrigger className="rounded-lg text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {encoders.map((e) => (
                    <SelectItem
                      key={e.codec}
                      value={e.codec}
                      disabled={!e.available && e.codec !== codec}
                    >
                      {e.label} · {e.encoder}
                      {!e.available && " (not in this ffmpeg build)"}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Skeleton className="h-9 w-full rounded-lg" />
            )}
            {encoder && !encoder.available && (
              <p className="text-xs text-red mt-1.5">
                This server&rsquo;s ffmpeg was built without {encoder.encoder},
                so this profile can&rsquo;t encode until it is.
              </p>
            )}
            {codec === "av1" && (
              <p className="text-xs text-gold mt-1.5">
                Check your players first. Many Plex and Jellyfin clients — older
                Rokus and Fire TV sticks, Apple TV, and plenty of smart TVs —
                can&rsquo;t direct-play AV1, so the server transcodes it on
                every play, which can cost far more CPU than the space is worth.
              </p>
            )}
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <LabelWithHelp label="CRF" help={crfHelp(codec, encoder)} />
              <Input
                type="number"
                min={encoder?.crf_min ?? 0}
                max={encoder?.crf_max ?? 51}
                value={crf}
                onChange={(e) => setCrf(Number(e.target.value))}
              />
            </div>
            <div>
              <LabelWithHelp label="Preset" help={presetHelp(codec)} />
              {encoder ? (
                <Select value={preset} onValueChange={setPreset}>
                  <SelectTrigger className="rounded-lg text-sm">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {encoder.presets.map((p) => (
                      <SelectItem key={p} value={p}>
                        {p}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Skeleton className="h-9 w-full rounded-lg" />
              )}
            </div>
          </div>
          <div>
            <LabelWithHelp
              label="Extra args"
              help={
                <>
                  Raw flags appended to the{" "}
                  <span className="font-mono">ffmpeg</span> command. Defaults to{" "}
                  <span className="font-mono">-c:a copy -c:s copy</span>, which
                  passes audio and subtitle streams through untouched so only
                  the video is re-encoded. Add flags here to tweak the output
                  (e.g. <span className="font-mono">-pix_fmt yuv420p10le</span>{" "}
                  for 10-bit).
                </>
              }
            />
            <Input
              className="font-mono text-sm"
              value={extra}
              onChange={(e) => setExtra(e.target.value)}
            />
            <p className="text-xs text-muted-dim mt-1">
              Audio/subtitle passthrough etc.
            </p>
          </div>
        </div>
        <DialogFooter className="px-6 py-4 border-t border-line flex justify-end gap-2">
          <Button variant="outline" onClick={onClose} className="rounded-xl">
            Cancel
          </Button>
          <Button
            onClick={() => void handleSave()}
            disabled={loading || !name || !encoder?.available}
            className="rounded-xl"
            style={{
              background:
                "linear-gradient(145deg, var(--brand), var(--brand-2))",
            }}
          >
            {loading ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
