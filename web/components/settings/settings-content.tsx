"use client";

import {
  useMutation,
  useQueryClient,
  useSuspenseQuery,
} from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { api, type Profile } from "@/lib/api";
import { AccountPanel } from "./account-panel";
import { CompatibilityPanel } from "./compatibility-panel";
import { EncodingPanel } from "./encoding-panel";
import { MetadataPanel } from "./metadata-panel";
import { DeleteProfileDialog, ProfileDialog } from "./profile-dialog";
import { ProfilesPanel } from "./profiles-panel";

export function SettingsContent() {
  const qc = useQueryClient();

  const { data: settings } = useSuspenseQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
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
  const { data: compatibilityProfilesData } = useSuspenseQuery({
    queryKey: ["compatibility-profiles"],
    queryFn: api.compatibilityProfiles,
    staleTime: 5 * 60_000,
  });
  const compatibilityProfiles = compatibilityProfilesData.profiles ?? [];

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
  const [defaultClientProfile, setDefaultClientProfile] = useState(
    settings.default_client_profile,
  );

  const [credPassword, setCredPassword] = useState("");
  const [credConfirm, setCredConfirm] = useState("");

  const settingsMutation = useMutation({
    mutationFn: () =>
      api.updateSettings({
        encode_window_start: windowStart,
        encode_window_end: windowEnd,
        scan_interval: `${scanIntervalHours}h0m0s`,
        scan_anchor: scanAnchor,
        probe_concurrency: probeConcurrency,
        default_client_profile: defaultClientProfile,
      }),
    onSuccess: () => {
      toast.success("Settings saved");
      qc.invalidateQueries({ queryKey: ["settings"] });
      qc.invalidateQueries({ queryKey: ["compatibility"] });
      qc.invalidateQueries({ queryKey: ["compatibility-stats"] });
    },
    onError: () => toast.error("Failed to save settings"),
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
        className="flex items-center gap-4 px-4 py-[14px] border-b border-line sm:px-7 sm:py-[18px]"
        style={{
          background: "rgba(22,22,22,.82)",
          backdropFilter: "blur(10px)",
        }}
      >
        <div>
          <div className="text-title font-bold tracking-tight">Settings</div>
          <div className="text-[0.82rem] text-muted-fg mt-px">
            Changes apply live — no restart
          </div>
        </div>
      </div>

      <div className="px-4 py-[22px] w-full pb-14 sm:px-7 sm:py-[26px]">
        <div className="grid grid-cols-2 gap-[18px] mb-[18px] max-sm:grid-cols-1">
          <EncodingPanel
            windowStart={windowStart}
            windowEnd={windowEnd}
            onWindowStartChange={setWindowStart}
            onWindowEndChange={setWindowEnd}
            probeConcurrency={probeConcurrency}
            onProbeConcurrencyChange={setProbeConcurrency}
            scanIntervalHours={scanIntervalHours}
            onScanIntervalHoursChange={setScanIntervalHours}
            scanAnchor={scanAnchor}
            onScanAnchorChange={setScanAnchor}
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

        <CompatibilityPanel
          profiles={compatibilityProfiles}
          value={defaultClientProfile}
          onChange={setDefaultClientProfile}
          onSave={() => settingsMutation.mutate()}
          isSaving={settingsMutation.isPending}
        />

        <ProfilesPanel
          profiles={profiles}
          onNew={() => setProfileDialog({ open: true, initial: null })}
          onEdit={(p) => setProfileDialog({ open: true, initial: p })}
          onSetDefault={(p) => defaultProfileMutation.mutate(p)}
          onDelete={setDeleteProfile}
          isSettingDefault={defaultProfileMutation.isPending}
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
