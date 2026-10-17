"use client";

import React from "react";
import { usePathname } from "next/navigation";
import { Navbar } from "@/components/layout/Navbar";
import SiteFooter from "@/components/site-footer";
import { cn } from "@/lib/utils";

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isHome = pathname === "/";

  return (
    <div className="flex min-h-screen flex-col bg-canvas text-ink">
      <Navbar />
      <div className={cn("flex-1", !isHome && "pt-16")}>{children}</div>
      <SiteFooter />
    </div>
  );
}
