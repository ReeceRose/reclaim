"use client";

import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { ConnectionLostScreen } from "@/components/connection-lost";
import { StatusScreen } from "@/components/status-screen";
import { ApiError, isNetworkError } from "@/lib/api";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const router = useRouter();
  const { reset: resetQueries } = useQueryErrorResetBoundary();
  const sessionExpired = error instanceof ApiError && error.status === 401;

  useEffect(() => {
    console.error(error);
  }, [error]);

  useEffect(() => {
    if (sessionExpired) {
      resetQueries();
      router.replace("/login");
    }
  }, [sessionExpired, resetQueries, router]);

  const retry = () => {
    resetQueries();
    reset();
  };

  if (sessionExpired) {
    return (
      <StatusScreen
        code="401"
        title="Session expired"
        description="Your login is no longer valid. Taking you to the login page…"
      />
    );
  }

  if (isNetworkError(error)) {
    return <ConnectionLostScreen onRetry={retry} />;
  }

  return (
    <StatusScreen
      code="Error"
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
        onClick={retry}
        className="inline-flex items-center justify-center h-10 px-5 rounded-xl text-sm font-semibold text-on-brand"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
          boxShadow: "0 4px 14px var(--brand-soft)",
        }}
      >
        Try again
      </button>
      <Link
        href="/"
        className="inline-flex items-center justify-center h-10 px-5 rounded-xl text-sm font-semibold border border-line text-text hover:bg-surface-2 transition-colors"
      >
        Back to overview
      </Link>
    </StatusScreen>
  );
}
