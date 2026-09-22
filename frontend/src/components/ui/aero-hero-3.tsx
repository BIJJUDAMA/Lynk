"use client";

import { cn } from "@/lib/utils";
import { useState } from "react";

export interface AeroCounterProps {
  initialCount?: number;
  label?: string;
  className?: string;
}

export function AeroCounter({
  initialCount = 0,
  label = "Active Verified Gigs",
  className,
}: AeroCounterProps) {
  const [count, setCount] = useState(initialCount);

  return (
    <div
      className={cn(
        "inline-flex items-center gap-3 rounded-md border border-border bg-card px-3 py-1.5 font-mono text-xs shadow-none",
        className
      )}
    >
      <span className="text-muted-foreground">{label}:</span>
      <span className="font-semibold text-foreground">{count}</span>
      <div className="flex items-center gap-1 border-l border-border pl-2">
        <button
          type="button"
          onClick={() => setCount((prev) => Math.max(0, prev - 1))}
          className="flex h-4 w-4 items-center justify-center rounded text-muted-foreground transition-transform hover:bg-muted hover:text-foreground active:scale-90"
          aria-label="Decrease count"
        >
          -
        </button>
        <button
          type="button"
          onClick={() => setCount((prev) => prev + 1)}
          className="flex h-4 w-4 items-center justify-center rounded text-muted-foreground transition-transform hover:bg-muted hover:text-foreground active:scale-90"
          aria-label="Increase count"
        >
          +
        </button>
      </div>
    </div>
  );
}

export const AeroHero3 = AeroCounter;
export const Component = AeroCounter;
export default AeroCounter;
