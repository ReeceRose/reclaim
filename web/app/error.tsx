"use client";

import Link from "next/link";
import { useEffect } from "react";
import { StatusScreen } from "@/components/status-screen";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <StatusScreen
      code="500"
      title="Something went wrong"
      description={
        <>
          An unexpected error interrupted this page. You can retry, or head back
          to the overview.
          {error.digest && (
            <span className="mt-3 block font-mono text-2xs text-muted-dim">
              ref: {error.digest}
            </span>
          )}
        </>
      }
    >
      <button
        type="button"
        onClick={() => reset()}
        className="inline-flex items-center justify-center h-10 px-5 rounded-[11px] text-sm font-semibold text-on-brand"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
          boxShadow: "0 4px 14px var(--brand-soft)",
        }}
      >
        Try again
      </button>
      <Link
        href="/"
        className="inline-flex items-center justify-center h-10 px-5 rounded-[11px] text-sm font-semibold border border-line text-text hover:bg-surface-2 transition-colors"
      >
        Back to overview
      </Link>
    </StatusScreen>
  );
}
