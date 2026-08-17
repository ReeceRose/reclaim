"use client";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { WebhookFormat } from "@/lib/api";
import { LabelWithHelp } from "./help-tip";

export const NOTIFY_DELAY_OPTIONS = [
  { value: "0", label: "As soon as possible" },
  { value: "60", label: "1 minute" },
  { value: "300", label: "5 minutes" },
  { value: "900", label: "15 minutes" },
  { value: "1800", label: "30 minutes" },
  { value: "3600", label: "1 hour" },
  { value: "7200", label: "2 hours" },
];

const WEBHOOK_FORMATS: { value: WebhookFormat; label: string }[] = [
  { value: "json", label: "Generic JSON" },
  { value: "discord", label: "Discord" },
  { value: "slack", label: "Slack" },
  { value: "ntfy", label: "ntfy" },
];

// The backend accepts any delay up to 24h, so a value set outside the UI (or a
// future option) still has to render as something.
function normalizeDelay(seconds: number): string {
  const value = String(seconds);
  return NOTIFY_DELAY_OPTIONS.some((o) => o.value === value) ? value : "900";
}

export function NotificationsPanel({
  enabled,
  onEnabledChange,
  delaySeconds,
  onDelaySecondsChange,
  webhookUrl,
  onWebhookUrlChange,
  webhookFormat,
  onWebhookFormatChange,
  onSave,
  isSaving,
  onTest,
  isTesting,
}: {
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  delaySeconds: number;
  onDelaySecondsChange: (v: number) => void;
  webhookUrl: string;
  onWebhookUrlChange: (v: string) => void;
  webhookFormat: WebhookFormat;
  onWebhookFormatChange: (v: WebhookFormat) => void;
  onSave: () => void;
  isSaving: boolean;
  onTest: () => void;
  isTesting: boolean;
}) {
  const delay = normalizeDelay(delaySeconds);

  return (
    <div
      className="border border-line rounded-(--radius) p-5 mt-5"
      style={{ background: "var(--surface)" }}
    >
      <div className="text-xs uppercase tracking-widest text-muted-fg font-bold mb-4">
        Notifications
      </div>

      <div className="flex items-start gap-2.5 mb-4">
        <Checkbox
          id="notify-enabled"
          checked={enabled}
          onCheckedChange={(v) => onEnabledChange(v === true)}
          className="mt-0.5"
        />
        <div className="min-w-0">
          <Label htmlFor="notify-enabled" className="text-xs font-semibold">
            Tell me when new re-encode candidates arrive
          </Label>
          <p className="text-xs text-muted-dim mt-1">
            Any newly-indexed file that isn&rsquo;t already HEVC — h264, mpeg4,
            VC-1, and the rest. Files already HEVC, or already queued, stay
            quiet.
          </p>
        </div>
      </div>

      <div className="mb-4">
        <LabelWithHelp
          label="Wait for the library to settle"
          help={
            <>
              New arrivals are collected until nothing new has landed for this
              long, then sent as{" "}
              <strong>one notification per show or movie</strong> — importing a
              whole season pings you once, not thirty times, and a second show
              arriving alongside it gets its own message rather than being mixed
              in. A steady trickle is sent anyway after four times this wait, so
              a long import still gets through.
            </>
          }
        />
        <Select
          value={delay}
          onValueChange={(v) => onDelaySecondsChange(Number(v))}
        >
          <SelectTrigger className="w-full rounded-xl text-sm sm:w-72">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {NOTIFY_DELAY_OPTIONS.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-dim mt-1.5">
          Batches arrivals so a season import is a single notification.
        </p>
      </div>

      <div className="mb-4 pt-4 border-t border-line">
        <LabelWithHelp
          label="Webhook (optional)"
          help={
            <>
              Where to send the notification besides the bell in the header.
              Pick the shape your receiver expects — <strong>Discord</strong>{" "}
              and <strong>Slack</strong> take their own JSON,{" "}
              <strong>ntfy</strong> takes plain text, and{" "}
              <strong>Generic JSON</strong> posts the full batch as JSON for
              anything else. Leave empty to keep notifications in-app only.
            </>
          }
        />
        <div className="flex items-center gap-2 flex-wrap">
          <Input
            type="url"
            inputMode="url"
            placeholder="https://discord.com/api/webhooks/…"
            value={webhookUrl}
            onChange={(e) => onWebhookUrlChange(e.target.value)}
            className="flex-1 min-w-0 sm:min-w-72"
          />
          <Select
            value={webhookFormat}
            onValueChange={(v) => onWebhookFormatChange(v as WebhookFormat)}
          >
            <SelectTrigger className="w-40 rounded-xl text-sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {WEBHOOK_FORMATS.map((f) => (
                <SelectItem key={f.value} value={f.value}>
                  {f.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <p className="text-xs text-muted-dim mt-1.5">
          The URL is stored on the server and survives restarts.
        </p>
      </div>

      <div className="flex items-center gap-2 flex-wrap">
        <Button
          onClick={onSave}
          disabled={isSaving}
          className="rounded-xl"
          style={{
            background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
          }}
        >
          {isSaving ? "Saving…" : "Save notifications"}
        </Button>
        <Button
          variant="outline"
          onClick={onTest}
          disabled={isTesting || webhookUrl.trim() === ""}
          className="rounded-xl"
        >
          {isTesting ? "Sending…" : "Send test"}
        </Button>
      </div>
    </div>
  );
}
