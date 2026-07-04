import type { Metadata } from "next";
import Link from "next/link";
import { StatusScreen } from "@/components/status-screen";

export const metadata: Metadata = {
  title: "Not found · Reclaim",
};

export default function NotFound() {
  return (
    <StatusScreen
      code="404"
      title="Page not found"
      description="That route doesn't exist. It may have moved, or the link was mistyped."
    >
      <Link
        href="/"
        className="inline-flex items-center justify-center h-10 px-5 rounded-xl text-sm font-semibold text-on-brand"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
          boxShadow: "0 4px 14px var(--brand-soft)",
        }}
      >
        Back to overview
      </Link>
    </StatusScreen>
  );
}
