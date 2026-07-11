"use client";

import { StatusScreen } from "@/components/status-screen";
import { Button } from "@/components/ui/button";

export function ConnectionLostScreen({ onRetry }: { onRetry: () => void }) {
  return (
    <StatusScreen
      code="Offline"
      title="Can't reach the server"
      description="The request never reached Reclaim — the server may be down, or your network or VPN connection dropped. Reconnect, then try again."
    >
      <Button
        onClick={onRetry}
        className="h-10 px-5 rounded-xl text-sm font-semibold text-on-brand"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
          boxShadow: "0 4px 14px var(--brand-soft)",
        }}
      >
        Try again
      </Button>
    </StatusScreen>
  );
}
