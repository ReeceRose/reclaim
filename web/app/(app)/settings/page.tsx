"use client";

import { Suspense } from "react";
import { SettingsContent } from "@/components/settings/settings-content";
import { SettingsSkeleton } from "@/components/settings/settings-skeleton";
import { Skeleton } from "@/components/ui/skeleton";

export default function Page() {
  return (
    <div className="flex flex-col min-w-0">
      <Suspense
        fallback={
          <>
            <div
              className="flex items-center gap-4 px-4 py-3.5 border-b border-line sm:px-7 sm:py-5"
              style={{
                background: "rgba(22,22,22,.82)",
                backdropFilter: "blur(10px)",
              }}
            >
              <div>
                <div className="text-title font-bold tracking-tight">
                  Settings
                </div>
                <Skeleton className="h-3 w-48 mt-1.5" />
              </div>
            </div>
            <SettingsSkeleton />
          </>
        }
      >
        <SettingsContent />
      </Suspense>
    </div>
  );
}
