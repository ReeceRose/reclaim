"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useQueryParams } from "@/hooks/use-query-params";

export default function Page() {
  const router = useRouter();
  const { get } = useQueryParams();
  const id = get("id");

  useEffect(() => {
    router.replace(id ? `/browse/file?id=${id}` : "/browse");
  }, [id, router]);

  return null;
}
