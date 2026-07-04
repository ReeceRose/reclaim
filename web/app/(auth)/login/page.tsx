"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError, api } from "@/lib/api";

export default function LoginPage() {
  const qc = useQueryClient();
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit() {
    setLoading(true);
    try {
      const { username: user } = await api.login(username, password);
      qc.setQueryData(["session"], {
        setup_complete: true,
        authenticated: true,
        username: user,
      });
      router.replace("/");
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Invalid credentials",
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void handleSubmit();
      }}
      className="text-left rounded-2xl p-6 border border-line"
      style={{ background: "var(--surface)" }}
    >
      <div className="mb-4">
        <Label className="block text-xs font-semibold mb-1.5">Username</Label>
        <Input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          required
        />
      </div>
      <div className="mb-4">
        <Label className="block text-xs font-semibold mb-1.5">Password</Label>
        <Input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
          required
        />
      </div>
      <Button
        type="submit"
        disabled={loading}
        className="w-full h-10 rounded-xl text-sm font-semibold text-on-brand"
        style={{
          background: "linear-gradient(145deg, var(--brand), var(--brand-2))",
          boxShadow: "0 4px 14px var(--brand-soft)",
        }}
      >
        {loading ? "Signing in…" : "Sign in"}
      </Button>
    </form>
  );
}
