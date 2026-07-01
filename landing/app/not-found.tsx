import { ArrowLeft } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { GitHubIcon } from "@/components/github-icon";
import { StatusScreen } from "@/components/status-screen";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "Page not found",
};

export default function NotFound() {
  return (
    <StatusScreen
      code="404"
      title="Page not found"
      description="That page doesn't exist. It may have moved, or the link was mistyped."
    >
      <Link
        href="/"
        className="inline-flex h-10 items-center gap-2 rounded-md bg-brand px-4 text-sm font-semibold text-on-brand transition-colors hover:bg-brand-2"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to home
      </Link>
      <a
        href={site.repo}
        target="_blank"
        rel="noreferrer"
        className="inline-flex h-10 items-center gap-2 rounded-md border border-line bg-surface px-4 text-sm font-medium text-text transition-colors hover:bg-surface-2"
      >
        <GitHubIcon className="h-4 w-4" />
        GitHub
      </a>
    </StatusScreen>
  );
}
