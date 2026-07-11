"use client";

import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";
import { ApiError, type Session } from "@/lib/api";
import { apiErrorMessage } from "@/lib/query-errors";

function handleQueryAuthError(qc: QueryClient, error: unknown) {
  if (error instanceof ApiError && error.status === 401) {
    qc.setQueryData<Session>(["session"], (old) =>
      old ? { ...old, authenticated: false, username: null } : old,
    );
    qc.invalidateQueries({ queryKey: ["session"] });
    return true;
  }
  return false;
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [client] = useState(() => {
    const qc = new QueryClient({
      queryCache: new QueryCache({
        onError: (error, query) => {
          if (handleQueryAuthError(qc, error)) return;
          if (query.queryKey[0] === "session") return;
          toast.error("Failed to load data", {
            description: apiErrorMessage(error),
          });
        },
      }),
      mutationCache: new MutationCache({
        onError: (error) => {
          if (handleQueryAuthError(qc, error)) return;
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
