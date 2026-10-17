"use client";

import { useEffect, type ReactNode } from "react";

export interface SmoothScrollProps {
  children: ReactNode;
}

/**
 * SmoothScroll provides clean, responsive momentum scrolling using native browser mechanics
 * without mousewheel hijacking or synthetic delays.
 */
export function SmoothScroll({ children }: SmoothScrollProps) {
  useEffect(() => {
    if (typeof window === "undefined") return;
    document.documentElement.style.scrollBehavior = "smooth";
    return () => {
      document.documentElement.style.scrollBehavior = "auto";
    };
  }, []);

  return <>{children}</>;
}

export default SmoothScroll;
