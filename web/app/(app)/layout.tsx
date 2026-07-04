"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { AppShell } from "@/components/app-shell";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();

  const { data: session, isLoading } = useQuery({
    queryKey: ["session"],
    queryFn: api.session,
    retry: false,
  });

  useEffect(() => {
    if (isLoading) return;
    if (!session?.setup_complete) router.replace("/setup");
    else if (!session?.authenticated) router.replace("/login");
  }, [session, isLoading, router]);

  if (isLoading || !session?.authenticated) {
    return (
      <div
        className="fixed inset-0 flex items-center justify-center"
        style={{ background: "var(--bg)" }}
      >
        <div className="flex flex-col items-center gap-3">
          <Skeleton className="w-8 h-8 rounded-lg" />
          <Skeleton className="w-24 h-2 rounded" />
        </div>
      </div>
    );
  }

  return <AppShell>{children}</AppShell>;
}
