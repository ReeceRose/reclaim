import { useQuery, useQueryClient } from "@tanstack/react-query";
import { usePathname, useRouter } from "next/navigation";
import { toast } from "sonner";
import { useUnreadCount } from "@/components/notification-panel";
import { useWS } from "@/hooks/use-ws";
import { api, type ScanProgress } from "@/lib/api";
import { formatInt, formatVersion, windowInfo } from "@/lib/format";

export function useShellData() {
  useWS();

  const { data: isScanning } = useQuery<boolean>({
    queryKey: ["scanning"],
    queryFn: () => false,
    initialData: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });
  const { data: scanProgress } = useQuery<ScanProgress | null>({
    queryKey: ["scan_progress"],
    queryFn: () => null,
    initialData: null,
    staleTime: Infinity,
    gcTime: Infinity,
  });
  const { data: wsConnected } = useQuery<boolean>({
    queryKey: ["ws_connected"],
    queryFn: () => true,
    initialData: true,
    staleTime: Infinity,
    gcTime: Infinity,
  });

  const pathname = usePathname();
  const router = useRouter();
  const qc = useQueryClient();

  const { data: session } = useQuery({
    queryKey: ["session"],
    queryFn: api.session,
    retry: false,
  });
  const { data: settings } = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings,
  });
  const { data: stats } = useQuery({
    queryKey: ["stats"],
    queryFn: api.stats,
    staleTime: 30_000,
  });
  const { data: runningJobs } = useQuery({
    queryKey: ["jobs", "running-count"],
    queryFn: () => api.jobs({ status: "running", limit: 1 }),
  });
  const { data: queuedJobs } = useQuery({
    queryKey: ["jobs", "queued-count"],
    queryFn: () => api.jobs({ status: "queued", limit: 1 }),
  });
  const { data: eventsData } = useQuery({
    queryKey: ["events"],
    queryFn: () => api.events({ limit: 50 }),
    staleTime: 30_000,
  });

  const unreadCount = useUnreadCount(eventsData?.items ?? []);

  const candidateCount = stats
    ? stats.by_codec
        .filter((c) => c.codec !== "hevc")
        .reduce((s, c) => s + c.file_count, 0)
    : null;

  const runningCount =
    runningJobs?.total_count ?? runningJobs?.items.length ?? 0;
  const queuedCount = queuedJobs?.total_count ?? queuedJobs?.items.length ?? 0;
  const queueBadge =
    runningCount + queuedCount > 0
      ? [
          runningCount > 0 ? String(runningCount) : "",
          queuedCount > 0 ? String(queuedCount) : "",
        ]
          .filter(Boolean)
          .join(" · ")
      : null;

  const candidateBadge =
    candidateCount != null
      ? candidateCount >= 1000
        ? `${(candidateCount / 1000).toFixed(1)}k`
        : String(candidateCount)
      : null;

  const navBadges: Record<string, string | null> = {
    "/candidates": candidateBadge,
    "/queue": queueBadge,
  };

  const encodeWindow = settings
    ? windowInfo(settings.encode_window_start, settings.encode_window_end)
    : null;
  const username = session?.username ?? "";
  const initials = username.slice(0, 2).toUpperCase() || "?";
  const version = session
    ? formatVersion(session.version, session.commit)
    : null;
  const scanProgressDetail = scanProgress
    ? [
        `${formatInt(scanProgress.files_processed)} indexed`,
        scanProgress.files_seen > scanProgress.files_processed
          ? `${formatInt(scanProgress.files_seen)} found`
          : null,
        scanProgress.errors > 0
          ? `${formatInt(scanProgress.errors)} errors`
          : null,
      ]
        .filter(Boolean)
        .join(" · ")
    : null;

  async function handleLogout() {
    try {
      await api.logout();
      qc.removeQueries({ queryKey: ["session"] });
      router.replace("/login");
    } catch {
      toast.error("Logout failed");
    }
  }

  return {
    pathname,
    isScanning: isScanning ?? false,
    wsConnected: wsConnected ?? true,
    scanProgressDetail,
    unreadCount,
    navBadges,
    settings,
    encodeWindow,
    username,
    initials,
    version,
    handleLogout,
  };
}
