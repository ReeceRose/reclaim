'use client';

import { Suspense } from 'react';
import { TvShowPageContent } from '@/components/browse/tv-show-page-content';
import { TvPageSkeleton } from '@/components/browse/tv-page-skeleton';

export default function Page() {
  return (
    <Suspense fallback={<TvPageSkeleton />}>
      <TvShowPageContent />
    </Suspense>
  );
}
