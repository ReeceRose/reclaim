"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError, api } from "@/lib/api";

export default function SetupPage() {
  const qc = useQueryClient();
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit() {
    setLoading(true);
    try {
      const { username: user } = await api.setup(username, password);
      qc.setQueryData(["session"], {
        setup_complete: true,
        authenticated: true,
        username: user,
      });
      router.replace("/");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Setup failed");
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
      <span className="inline-block mb-4 px-2 py-0.5 rounded text-xs font-bold uppercase tracking-widest text-brand bg-brand-soft border border-brand-line">
        First-run setup
      </span>
      <div className="mb-4">
        <Label className="block text-xs font-semibold mb-1.5">
          Create a username
        </Label>
        <Input
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          required
        />
      </div>
      <div className="mb-4">
        <Label className="block text-xs font-semibold mb-1.5">
          Create a password
        </Label>
        <Input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="new-password"
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
        {loading ? "Setting up…" : "Complete setup"}
      </Button>
      <p className="text-xs text-muted-dim mt-4">
        Stored bcrypt-hashed in the database. No password lives in your compose
        file.
      </p>
    </form>
  );
}
