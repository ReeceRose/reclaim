// Central place for the marketing copy / links so swapping the domain or repo
// is a one-line change.

export const site = {
  name: "Reclaim",
  url: "https://reclaim.reecerose.com",
  tagline: "Reclaim disk space from your media library — safely.",
  description:
    "Self-hosted codec audit and re-encode tool for homelabs. Point it at the movie and TV folders Plex, Jellyfin, or Emby already use, rank files by predicted HEVC savings, see which are better re-downloaded than re-encoded, and manually queue overnight ffmpeg jobs.",
  repo: "https://github.com/reecerose/reclaim",
  docker: "ghcr.io/reecerose/reclaim:latest",
  docsBase: "https://github.com/reecerose/reclaim/blob/main/docs",
} as const;

export const nav = [
  { label: "Features", href: "#features" },
  { label: "How it works", href: "#how" },
  { label: "Install", href: "#install" },
  { label: "Docs", href: `${site.repo}/blob/main/docs/DOCKER.md`, external: true },
];
