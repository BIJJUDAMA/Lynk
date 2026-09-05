"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function EmployerJobsPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/activity?tab=postings");
  }, [router]);

  return null;
}
