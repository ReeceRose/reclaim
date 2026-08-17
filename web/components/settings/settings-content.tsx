"use client";

import {
  useMutation,
  useQueryClient,
  useSuspenseQuery,
} from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import {
  api,
  type ClockFormat,
  type Profile,
  type Settings,
  type WebhookFormat,
} from "@/lib/api";
import { AccountPanel } from "./account-panel";
import { EncodingPanel } from "./encoding-panel";
import { LibraryPanel, RETENTION_OPTIONS } from "./library-panel";
import { MetadataPanel } from "./metadata-panel";
import { NotificationsPanel } from "./notifications-panel";
import { DeleteProfileDialog, ProfileDialog } from "./profile-dialog";
import { ProfilesPanel } from "./profiles-panel";

export function SettingsContent() {
  const qc = useQueryClient();

  const { data: settings } = useSuspenseQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
    refetchInterval: 60_000,
  });
  const { data: session } = useSuspenseQuery({
    queryKey: ["session"],
    queryFn: api.session,
  });
  const { data: profilesData } = useSuspenseQuery({
    queryKey: ["profiles"],
    queryFn: api.profiles,
    staleTime: 30_000,
  });
  const profiles = profilesData.items ?? [];

  const [timezone, setTimezone] = useState(settings.timezone);
  const [windowStart, setWindowStart] = useState(settings.encode_window_start);
  const [windowEnd, setWindowEnd] = useState(settings.encode_window_end);
  const [scanIntervalHours, setScanIntervalHours] = useState(() => {
    const m = settings.scan_interval.match(/^(\d+)h/);
    return m ? parseInt(m[1], 10) : 24;
  });
  const [scanAnchor, setScanAnchor] = useState(settings.scan_anchor ?? "00:00");
  const [probeConcurrency, setProbeConcurrency] = useState(
    settings.probe_concurrency,
  );
  const [oversizeThreshold, setOversizeThreshold] = useState(
    settings.oversize_threshold,
  );
  const [missingRetention, setMissingRetention] = useState(
    settings.missing_retention ?? "0",
  );

  const [notifyEnabled, setNotifyEnabled] = useState(settings.notify_enabled);
  const [notifyDelay, setNotifyDelay] = useState(settings.notify_delay_seconds);
  const [notifyWebhookUrl, setNotifyWebhookUrl] = useState(
    settings.notify_webhook_url,
  );
  const [notifyWebhookFormat, setNotifyWebhookFormat] = useState<WebhookFormat>(
    settings.notify_webhook_format,
  );

  const [credPassword, setCredPassword] = useState("");
  const [credConfirm, setCredConfirm] = useState("");

  const settingsMutation = useMutation({
    mutationFn: () =>
      api.updateSettings({
        timezone,
        encode_window_start: windowStart,
        encode_window_end: windowEnd,
        scan_interval: `${scanIntervalHours}h0m0s`,
        scan_anchor: scanAnchor,
        probe_concurrency: probeConcurrency,
        oversize_threshold: oversizeThreshold,
      }),
    onSuccess: () => {
      toast.success("Settings saved");
      qc.invalidateQueries({ queryKey: ["settings"] });
    },
    onError: () => toast.error("Failed to save settings"),
  });

  const retentionMutation = useMutation({
    mutationFn: (value: string) =>
      api.updateSettings({ missing_retention: value }),
    onMutate: (value: string) => {
      const previous = missingRetention;
      setMissingRetention(value);
      return previous;
    },
    onSuccess: (_data, value) => {
      toast.success(
        value === "0"
          ? "Missing files will be kept indefinitely"
          : `Missing files will be removed after ${RETENTION_OPTIONS.find((o) => o.value === value)?.label ?? value}`,
      );
      qc.invalidateQueries({ queryKey: ["settings"] });
    },
    onError: (_err, _value, previous) => {
      setMissingRetention(previous ?? "0");
      toast.error("Failed to save retention period");
    },
  });

  const notifyMutation = useMutation({
    mutationFn: () =>
      api.updateSettings({
        notify_enabled: notifyEnabled,
        notify_delay_seconds: notifyDelay,
        notify_webhook_url: notifyWebhookUrl.trim(),
        notify_webhook_format: notifyWebhookFormat,
      }),
    onSuccess: (updated) => {
      toast.success("Notification settings saved");
      qc.setQueryData<Settings>(["settings"], updated);
    },
    onError: (err: Error) =>
      toast.error("Failed to save notification settings", {
        description: err.message,
      }),
  });

  const notifyTestMutation = useMutation({
    mutationFn: () =>
      api.testNotification({
        notify_webhook_url: notifyWebhookUrl.trim(),
        notify_webhook_format: notifyWebhookFormat,
      }),
    onSuccess: () => toast.success("Test notification sent"),
    onError: (err: Error) =>
      toast.error("Test notification failed", { description: err.message }),
  });

  const clockFormatMutation = useMutation({
    mutationFn: (value: ClockFormat) =>
      api.updateSettings({ clock_format: value }),
    onMutate: (value: ClockFormat) => {
      const previous = qc.getQueryData<Settings>(["settings"]);
      if (previous) {
        qc.setQueryData<Settings>(["settings"], {
          ...previous,
          clock_format: value,
        });
      }
      return previous;
    },
    onSuccess: (updated) => qc.setQueryData<Settings>(["settings"], updated),
    onError: (_err, _value, previous) => {
      if (previous) qc.setQueryData<Settings>(["settings"], previous);
      toast.error("Failed to save clock format");
    },
  });

  const pruneMutation = useMutation({
    mutationFn: () => api.pruneMissing(),
    onSuccess: (result) => {
      toast.success(
        result.deleted === 1
          ? "Removed 1 missing file"
          : `Removed ${result.deleted} missing files`,
      );
      qc.invalidateQueries({ queryKey: ["settings"] });
      qc.invalidateQueries({ queryKey: ["stats"] });
      qc.invalidateQueries({ queryKey: ["files"] });
      qc.invalidateQueries({ queryKey: ["library"] });
      qc.invalidateQueries({ queryKey: ["browse"] });
      qc.invalidateQueries({ queryKey: ["events"] });
    },
    onError: () => toast.error("Failed to remove missing files"),
  });

  const credMutation = useMutation({
    mutationFn: () =>
      api.changeCredentials(session.username ?? "", credPassword),
    onSuccess: () => {
      toast.success("Credentials updated");
      setCredPassword("");
      setCredConfirm("");
    },
    onError: () => toast.error("Failed to update credentials"),
  });

  const refreshMetaMutation = useMutation({
    mutationFn: () => api.refreshMetadata(),
    onSuccess: () => toast.success("Metadata refresh queued"),
    onError: () => toast.error("Refresh failed"),
  });

  const deleteProfileMutation = useMutation({
    mutationFn: (id: number) => api.deleteProfile(id),
    onSuccess: () => {
      toast.success("Profile deleted");
      qc.invalidateQueries({ queryKey: ["profiles"] });
    },
    onError: () => toast.error("Delete failed"),
  });

  const defaultProfileMutation = useMutation({
    mutationFn: ({ id, ...profile }: Profile) =>
      api.updateProfile(id, { ...profile, is_default: true }),
    onSuccess: (profile) => {
      toast.success(`"${profile.name}" is now the default`);
      qc.invalidateQueries({ queryKey: ["profiles"] });
    },
    onError: () => toast.error("Failed to update default profile"),
  });

  const [profileDialog, setProfileDialog] = useState<{
    open: boolean;
    initial: Partial<Profile> | null;
  }>({
    open: false,
    initial: null,
  });
  const [deleteProfile, setDeleteProfile] = useState<Profile | null>(null);

  function handleCredSave() {
    if (credPassword !== credConfirm) {
      toast.error("Passwords do not match");
      return;
    }
    credMutation.mutate();
  }

  return (
    <>
      <div
        className="flex items-center gap-4 px-4 py-3.5 border-b border-line sm:px-7 sm:py-5"
        style={{
          background: "rgba(22,22,22,.82)",
          backdropFilter: "blur(10px)",
        }}
      >
        <div>
          <div className="text-title font-bold tracking-tight">Settings</div>
          <div className="text-sm text-muted-fg mt-px">
            Changes apply live — no restart
          </div>
        </div>
      </div>

      <div className="px-4 py-6 w-full pb-14 sm:px-7 sm:py-7">
        <div className="grid grid-cols-2 gap-5 mb-5 max-sm:grid-cols-1">
          <EncodingPanel
            timezone={timezone}
            savedTimezone={settings.timezone}
            onTimezoneChange={setTimezone}
            serverTime={settings.server_time}
            windowState={settings}
            windowStart={windowStart}
            windowEnd={windowEnd}
            onWindowStartChange={setWindowStart}
            onWindowEndChange={setWindowEnd}
            probeConcurrency={probeConcurrency}
            onProbeConcurrencyChange={setProbeConcurrency}
            oversizeThreshold={oversizeThreshold}
            onOversizeThresholdChange={setOversizeThreshold}
            scanIntervalHours={scanIntervalHours}
            onScanIntervalHoursChange={setScanIntervalHours}
            scanAnchor={scanAnchor}
            onScanAnchorChange={setScanAnchor}
            onClockFormatChange={(v) => clockFormatMutation.mutate(v)}
            onSave={() => settingsMutation.mutate()}
            isSaving={settingsMutation.isPending}
          />
          <AccountPanel
            username={session.username ?? ""}
            credPassword={credPassword}
            credConfirm={credConfirm}
            onCredPasswordChange={setCredPassword}
            onCredConfirmChange={setCredConfirm}
            onSave={handleCredSave}
            isSaving={credMutation.isPending}
            moviesPath={settings.movies_path}
            tvPath={settings.tv_path}
          />
        </div>

        <ProfilesPanel
          profiles={profiles}
          onNew={() => setProfileDialog({ open: true, initial: null })}
          onEdit={(p) => setProfileDialog({ open: true, initial: p })}
          onSetDefault={(p) => defaultProfileMutation.mutate(p)}
          onDelete={setDeleteProfile}
          isSettingDefault={defaultProfileMutation.isPending}
        />

        <LibraryPanel
          retention={missingRetention}
          onRetentionChange={(v) => retentionMutation.mutate(v)}
          missing={settings.missing_files}
          onPurge={() => pruneMutation.mutate()}
          isPurging={pruneMutation.isPending}
        />

        <NotificationsPanel
          enabled={notifyEnabled}
          onEnabledChange={setNotifyEnabled}
          delaySeconds={notifyDelay}
          onDelaySecondsChange={setNotifyDelay}
          webhookUrl={notifyWebhookUrl}
          onWebhookUrlChange={setNotifyWebhookUrl}
          webhookFormat={notifyWebhookFormat}
          onWebhookFormatChange={setNotifyWebhookFormat}
          onSave={() => notifyMutation.mutate()}
          isSaving={notifyMutation.isPending}
          onTest={() => notifyTestMutation.mutate()}
          isTesting={notifyTestMutation.isPending}
        />

        <MetadataPanel
          tmdbConfigured={!!settings.tmdb_configured}
          onRefresh={() => refreshMetaMutation.mutate()}
          isRefreshing={refreshMetaMutation.isPending}
        />
      </div>

      <ProfileDialog
        key={`${String(profileDialog.open)}-${profileDialog.initial?.id ?? "new"}`}
        open={profileDialog.open}
        onClose={() => setProfileDialog({ open: false, initial: null })}
        initial={profileDialog.initial}
      />
      <DeleteProfileDialog
        profile={deleteProfile}
        onClose={() => setDeleteProfile(null)}
        onConfirm={() =>
          deleteProfile && deleteProfileMutation.mutate(deleteProfile.id)
        }
      />
    </>
  );
}
