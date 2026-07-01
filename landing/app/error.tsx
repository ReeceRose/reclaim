"use client";

import { ArrowLeft, RotateCw } from "lucide-react";
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
          An unexpected error interrupted this page. Try again, or head back
          home.
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
        className="inline-flex h-10 items-center gap-2 rounded-md bg-brand px-4 text-sm font-semibold text-on-brand transition-colors hover:bg-brand-2"
      >
        <RotateCw className="h-4 w-4" />
        Try again
      </button>
      <Link
        href="/"
        className="inline-flex h-10 items-center gap-2 rounded-md border border-line bg-surface px-4 text-sm font-medium text-text transition-colors hover:bg-surface-2"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to home
      </Link>
    </StatusScreen>
  );
}
