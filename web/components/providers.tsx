'use client';

import { QueryCache, QueryClient, QueryClientProvider, MutationCache } from '@tanstack/react-query';
import { Toaster } from '@/components/ui/sonner';
import { ApiError } from '@/lib/api';
import { useState } from 'react';

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(() => {
    const qc = new QueryClient({
      queryCache: new QueryCache({
        onError: (error) => {
          if (error instanceof ApiError && error.status === 401) {
            qc.invalidateQueries({ queryKey: ['session'] });
          }
        },
      }),
      mutationCache: new MutationCache({
        onError: (error) => {
          if (error instanceof ApiError && error.status === 401) {
            qc.invalidateQueries({ queryKey: ['session'] });
          }
        },
      }),
      defaultOptions: {
        queries: { retry: 1, staleTime: 15_000 },
      },
    });
    return qc;
  });

  return (
    <QueryClientProvider client={client}>
      {children}
      <Toaster richColors position="bottom-right" />
    </QueryClientProvider>
  );
}
