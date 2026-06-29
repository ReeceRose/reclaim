import {
  ArrowRight,
  BarChart3,
  Clock,
  Download,
  Eye,
  ExternalLink,
  FolderSearch,
  HardDrive,
  Image,
  ListOrdered,
  Moon,
  Shield,
  Zap,
} from "lucide-react";
import { CopyButton } from "@/components/copy-button";
import { site } from "@/lib/site";

const features = [
  {
    icon: FolderSearch,
    title: "Scan your library",
    body: "Walks your movie and TV folders with ffprobe. Skips unchanged files on rescans and detects renames via fingerprinting.",
    accent: "var(--brand)",
  },
  {
    icon: BarChart3,
    title: "Savings that adapt",
    body: "Ranks candidates by predicted HEVC savings. Reclaim starts with conservative defaults, then recalculates estimates from your own completed encodes.",
    accent: "var(--sky)",
  },
  {
    icon: Eye,
    title: "Browse every file",
    body: "The Library view shows all scanned files, including already-HEVC, skipped, and missing items, with a clear reason each one is or isn't a candidate.",
    accent: "var(--green)",
  },
  {
    icon: Download,
    title: "Re-encode or re-download",
    body: "Not every file is worth re-encoding. Per-file codec, bitrate, and resolution help you spot the bloated or low-quality rips better replaced from a cleaner source.",
    accent: "var(--rose)",
  },
  {
    icon: ListOrdered,
    title: "Queue manually",
    body: "Select files, pick an encode profile, and confirm before anything runs. No surprise batch conversions.",
    accent: "var(--violet)",
  },
  {
    icon: Moon,
    title: "Encode overnight",
    body: "Jobs run inside a configurable time window — default midnight to 6 AM. A running job finishes even if the window closes.",
    accent: "var(--gold)",
  },
  {
    icon: Shield,
    title: "Verify before swap",
    body: "Every encode is checked (duration ±1 s, stream counts, resolution) before the original is touched.",
    accent: "var(--green)",
  },
  {
    icon: Image,
    title: "TMDB artwork",
    body: "Optional TMDB integration fetches poster and backdrop images for your library. Browse cards show artwork automatically — no manual tagging.",
    accent: "var(--sky)",
  },
  {
    icon: HardDrive,
    title: "Single container",
    body: "Go API + embedded web UI + ffmpeg/ffprobe. No database server, Redis, or sidecar services.",
    accent: "var(--rose)",
  },
];

const steps = [
  {
    n: "01",
    title: "Scan",
    body: "Reclaim walks MOVIES_PATH and TV_PATH, probes each video file, and records codec, resolution, bitrate, size, and fingerprint.",
  },
  {
    n: "02",
    title: "Rank & review",
    body: "Files are sorted by predicted HEVC savings. As jobs complete, Reclaim recalculates estimates from your library's results so future rankings stay grounded in real outcomes.",
  },
  {
    n: "03",
    title: "Queue",
    body: "Select candidates, pick a profile, and confirm. Jobs are created but won't run until the encode window opens.",
  },
  {
    n: "04",
    title: "Encode",
    body: "ffmpeg writes a .reclaim-tmp file. On pass: atomic swap. On fail: original untouched, temp kept for inspection.",
  },
];

const does = [
  "Scans mounted library folders directly",
  "Ranks candidates by savings estimates that improve over time",
  "Fetches TMDB poster art for movies and TV shows (optional)",
  "Helps spot files better re-downloaded than re-encoded",
  "Replaces files in-place after verification",
  "Runs encodes in a configurable overnight window",
];

const doesNot = [
  "Integrate with Sonarr, Radarr, Plex, Jellyfin, or Emby APIs",
  "Auto-encode your whole library",
  "Use GPU/NVENC hardware encoding (CPU libx265 only)",
  "Pause for active streams (time window only)",
];

const presets = [
  { name: "medium", speed: "~0.5–1× realtime", hour: "1–2 hours" },
  { name: "fast", speed: "~2–3× realtime", hour: "20–30 min" },
  { name: "ultrafast", speed: "~8–10× realtime", hour: "6–8 min" },
];

export function Features() {
  return (
    <section id="features" className="border-b border-line py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div className="max-w-2xl">
          <p className="text-2xs font-bold uppercase tracking-[0.13em] text-brand">
            Features
          </p>
          <h2 className="mt-2 text-3xl font-extrabold tracking-tight sm:text-4xl">
            Built for large homelab libraries
          </h2>
          <p className="mt-3 text-base leading-relaxed text-muted-fg">
            Mixed codecs, tens of terabytes, and a need to chip away safely — not
            batch-convert everything at once.
          </p>
        </div>

        <div className="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f) => (
            <div
              key={f.title}
              className="rounded-[var(--radius)] border border-line bg-surface p-5 transition-colors hover:border-line/80 hover:bg-surface/80"
            >
              <div
                className="mb-4 inline-flex h-10 w-10 items-center justify-center rounded-lg border"
                style={{
                  color: f.accent,
                  borderColor: `color-mix(in srgb, ${f.accent} 30%, transparent)`,
                  background: `color-mix(in srgb, ${f.accent} 10%, transparent)`,
                }}
              >
                <f.icon className="h-5 w-5" />
              </div>
              <h3 className="text-base font-bold">{f.title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-muted-fg">{f.body}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function HowItWorks() {
  return (
    <section id="how" className="border-b border-line bg-surface/40 py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div className="max-w-2xl">
          <p className="text-2xs font-bold uppercase tracking-[0.13em] text-brand">
            How it works
          </p>
          <h2 className="mt-2 text-3xl font-extrabold tracking-tight sm:text-4xl">
            Scan → rank → queue → encode
          </h2>
          <p className="mt-3 text-base leading-relaxed text-muted-fg">
            A manual-first workflow. You stay in control at every step.
          </p>
        </div>

        <div className="mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {steps.map((s) => (
            <div
              key={s.n}
              className="relative rounded-[var(--radius)] border border-line bg-surface p-5"
            >
              <span className="font-mono text-2xs font-bold tracking-widest text-brand">
                {s.n}
              </span>
              <h3 className="mt-3 text-lg font-bold">{s.title}</h3>
              <p className="mt-2 text-sm leading-relaxed text-muted-fg">{s.body}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

export function Safety() {
  return (
    <section className="border-b border-line py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div className="grid items-center gap-10 lg:grid-cols-2">
          <div>
            <p className="text-2xs font-bold uppercase tracking-[0.13em] text-green">
              Safety model
            </p>
            <h2 className="mt-2 text-3xl font-extrabold tracking-tight sm:text-4xl">
              Your originals are never deleted first
            </h2>
            <p className="mt-4 text-base leading-relaxed text-muted-fg">
              Reclaim encodes to a temporary file, verifies the output, then
              atomically swaps. A crash mid-swap is recovered on next boot.
            </p>
          </div>

          <div className="rounded-[var(--radius)] border border-line bg-surface p-5 font-mono text-sm">
            <div className="mb-4 text-2xs font-bold uppercase tracking-wider text-muted-fg">
              Replace sequence
            </div>
            <div className="space-y-3 text-muted-fg">
              <div className="flex items-start gap-3">
                <span className="mt-0.5 text-brand">1.</span>
                <span>
                  <span className="text-text">original</span>
                  {" → "}
                  <span className="text-gold">original.reclaim-backup</span>
                  <span className="ml-2 text-2xs text-muted-dim">(rename, atomic)</span>
                </span>
              </div>
              <div className="flex items-start gap-3">
                <span className="mt-0.5 text-brand">2.</span>
                <span>
                  <span className="text-sky">.reclaim-tmp</span>
                  {" → "}
                  <span className="text-text">original</span>
                  <span className="ml-2 text-2xs text-muted-dim">(rename, atomic)</span>
                </span>
              </div>
              <div className="flex items-start gap-3">
                <span className="mt-0.5 text-brand">3.</span>
                <span>
                  delete <span className="text-gold">original.reclaim-backup</span>
                </span>
              </div>
            </div>
            <p className="mt-5 border-t border-line-soft pt-4 text-xs leading-relaxed text-muted-dim">
              On failure: job marked failed, temp kept for inspection, original
              untouched. On boot: orphaned temps cleaned, interrupted backups
              restored.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

export function Comparison() {
  return (
    <section className="border-b border-line bg-surface/40 py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div className="max-w-2xl">
          <p className="text-2xs font-bold uppercase tracking-[0.13em] text-brand">
            Scope
          </p>
          <h2 className="mt-2 text-3xl font-extrabold tracking-tight sm:text-4xl">
            What Reclaim does — and doesn&apos;t
          </h2>
        </div>

        <div className="mt-10 grid gap-5 sm:grid-cols-2">
          <div className="rounded-[var(--radius)] border border-green/20 bg-green-soft/30 p-5">
            <h3 className="flex items-center gap-2 text-sm font-bold text-green">
              <Zap className="h-4 w-4" />
              Does
            </h3>
            <ul className="mt-4 space-y-3">
              {does.map((item) => (
                <li key={item} className="flex items-start gap-2.5 text-sm text-muted-fg">
                  <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-green" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
          <div className="rounded-[var(--radius)] border border-line bg-surface p-5">
            <h3 className="flex items-center gap-2 text-sm font-bold text-muted-fg">
              <Clock className="h-4 w-4" />
              Does not
            </h3>
            <ul className="mt-4 space-y-3">
              {doesNot.map((item) => (
                <li key={item} className="flex items-start gap-2.5 text-sm text-muted-fg">
                  <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-slate" />
                  {item}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}

export function Install() {
  const composeSnippet = `# docker-compose.yml — edit media paths and TZ first
services:
  reclaim:
    image: ${site.docker}
    ports:
      - "8080:8080"
    volumes:
      - /path/to/movies:/movies:rw
      - /path/to/tv:/tv:rw
      - reclaim-data:/data
    environment:
      MOVIES_PATH: /movies
      TV_PATH: /tv
      DB_PATH: /data/reclaim.db
      TZ: America/New_York

volumes:
  reclaim-data:`;

  const runCmd = "docker compose up --build -d";

  return (
    <section id="install" className="border-b border-line py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div className="max-w-2xl">
          <p className="text-2xs font-bold uppercase tracking-[0.13em] text-brand">
            Install
          </p>
          <h2 className="mt-2 text-3xl font-extrabold tracking-tight sm:text-4xl">
            Up and running in minutes
          </h2>
          <p className="mt-3 text-base leading-relaxed text-muted-fg">
            Single container. Mount your library read-write, open port 8080, create
            your login on first boot.
          </p>
        </div>

        <div className="mt-10 grid gap-5 lg:grid-cols-[1.2fr_1fr]">
          <div className="overflow-hidden rounded-[var(--radius)] border border-line bg-surface">
            <div className="flex items-center justify-between border-b border-line px-4 py-2.5">
              <span className="font-mono text-xs text-muted-fg">docker-compose.yml</span>
              <CopyButton text={composeSnippet} label="Copy docker-compose.yml" />
            </div>
            <pre className="overflow-x-auto p-4 font-mono text-xs leading-relaxed text-muted-fg">
              <code>{composeSnippet}</code>
            </pre>
          </div>

          <div className="flex flex-col gap-4">
            <div className="rounded-[var(--radius)] border border-line bg-surface p-5">
              <h3 className="text-sm font-bold">1. Pull and start</h3>
              <div className="mt-3 flex items-center justify-between gap-3 rounded-md border border-line bg-bg px-3 py-2.5">
                <code className="font-mono text-xs text-text">{runCmd}</code>
                <CopyButton text={runCmd} label="Copy docker compose command" />
              </div>
            </div>

            <div className="rounded-[var(--radius)] border border-line bg-surface p-5">
              <h3 className="text-sm font-bold">2. Open the UI</h3>
              <p className="mt-2 text-sm text-muted-fg">
                Navigate to{" "}
                <code className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-xs text-text">
                  http://&lt;nas-ip&gt;:8080
                </code>
                , create your login, and let the first scan run.
              </p>
            </div>

            <div className="rounded-[var(--radius)] border border-line bg-surface p-5">
              <h3 className="text-sm font-bold">3. Read the docs</h3>
              <p className="mt-2 text-sm text-muted-fg">
                Full deployment guide for Unraid, Synology, and standalone binary.
              </p>
              <a
                href={`${site.docsBase}/DOCKER.md`}
                target="_blank"
                rel="noreferrer"
                className="mt-3 inline-flex items-center gap-1.5 text-sm font-semibold text-brand hover:underline"
              >
                docs/DOCKER.md
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export function Throughput() {
  return (
    <section className="border-b border-line bg-surface/40 py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div className="max-w-2xl">
          <p className="text-2xs font-bold uppercase tracking-[0.13em] text-gold">
            Throughput
          </p>
          <h2 className="mt-2 text-3xl font-extrabold tracking-tight sm:text-4xl">
            CPU x265 is slow by design
          </h2>
          <p className="mt-3 text-base leading-relaxed text-muted-fg">
            Reclaim is meant to chip away safely over weeks and months of overnight
            windows — not batch-convert a 20,000-file library in a weekend.
          </p>
        </div>

        <div className="mt-10 overflow-hidden rounded-[var(--radius)] border border-line">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-line bg-surface">
                <th className="px-5 py-3 font-bold text-muted-fg">Preset</th>
                <th className="px-5 py-3 font-bold text-muted-fg">Typical speed</th>
                <th className="hidden px-5 py-3 font-bold text-muted-fg sm:table-cell">
                  1-hour HD file
                </th>
              </tr>
            </thead>
            <tbody>
              {presets.map((p) => (
                <tr key={p.name} className="border-b border-line-soft last:border-0">
                  <td className="px-5 py-3.5">
                    <code className="rounded-md border border-line bg-surface-2 px-2 py-0.5 font-mono text-xs">
                      {p.name}
                    </code>
                  </td>
                  <td className="px-5 py-3.5 text-muted-fg tnum">{p.speed}</td>
                  <td className="hidden px-5 py-3.5 text-muted-fg tnum sm:table-cell">
                    {p.hour}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

export function CTA() {
  return (
    <section className="py-20 sm:py-24">
      <div className="mx-auto max-w-6xl px-5 sm:px-7">
        <div
          className="relative overflow-hidden rounded-[18px] border border-line px-6 py-12 text-center sm:px-10 sm:py-14"
          style={{
            background:
              "radial-gradient(120% 150% at 50% 0%, var(--brand-soft), transparent 55%), var(--surface)",
          }}
        >
          <div className="scanlines pointer-events-none absolute inset-0" />
          <div className="relative">
            <h2 className="text-2xl font-extrabold tracking-tight sm:text-3xl">
              Ready to reclaim some space?
            </h2>
            <p className="mx-auto mt-3 max-w-lg text-sm leading-relaxed text-muted-fg sm:text-base">
              Clone the repo, point it at your library, and see how much you could
              save — before encoding a single file.
            </p>
            <div className="mt-7 flex flex-wrap items-center justify-center gap-3">
              <a
                href={site.repo}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-11 items-center gap-2 rounded-lg bg-brand px-5 text-sm font-semibold text-on-brand transition-colors hover:bg-brand-2"
              >
                View on GitHub
                <ArrowRight className="h-4 w-4" />
              </a>
              <a
                href={`${site.docsBase}/DOCKER.md`}
                target="_blank"
                rel="noreferrer"
                className="inline-flex h-11 items-center gap-2 rounded-lg border border-line bg-surface px-5 text-sm font-semibold text-text transition-colors hover:bg-surface-2"
              >
                Deployment guide
                <ExternalLink className="h-4 w-4 text-muted-fg" />
              </a>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export function Footer() {
  return (
    <footer className="border-t border-line py-10">
      <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-6 px-5 sm:flex-row sm:px-7">
        <div className="flex items-center gap-2.5">
          <span className="text-sm font-extrabold tracking-tight">
            <span className="text-text">Re</span>
            <span className="text-brand">claim</span>
          </span>
          <span className="text-xs text-muted-dim">· self-hosted · open source</span>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-5 text-sm text-muted-fg">
          <a href={site.repo} target="_blank" rel="noreferrer" className="hover:text-text">
            GitHub
          </a>
          <a
            href={`${site.docsBase}/DOCKER.md`}
            target="_blank"
            rel="noreferrer"
            className="hover:text-text"
          >
            Docs
          </a>
          <a
            href={`${site.docsBase}/API.md`}
            target="_blank"
            rel="noreferrer"
            className="hover:text-text"
          >
            API
          </a>
        </div>
      </div>
    </footer>
  );
}
