"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function ApplicationsPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/activity?tab=applications");
  }, [router]);

  return null;
}
