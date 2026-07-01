"use client";

import { Suspense } from "react";
import { TvPageSkeleton } from "@/components/browse/tv-page-skeleton";
import { TvShowPageContent } from "@/components/browse/tv-show-page-content";

export default function Page() {
  return (
    <Suspense fallback={<TvPageSkeleton />}>
      <TvShowPageContent />
    </Suspense>
  );
}
