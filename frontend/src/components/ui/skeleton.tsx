import { cn } from "@/lib/utils";

/**
 * Skeleton renders a zero-CLS animated placeholder that matches the layout
 * dimensions of real content before it loads.
 *
 * Usage:
 *   // Match the exact height and width of the real element:
 *   <Skeleton className="h-48 w-full rounded-xl" />
 */
export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("animate-pulse rounded bg-surface border border-line/40", className)}
      aria-hidden="true"
      {...props}
    />
  );
}

/**
 * JobCardSkeleton renders a placeholder matching a JobCard's dimensions
 * for use in the jobs listing grid while data is loading.
 */
export function JobCardSkeleton() {
  return (
    <div className="rounded-xl border border-line bg-card p-6 space-y-4">
      <Skeleton className="h-5 w-3/4" />
      <Skeleton className="h-4 w-1/3" />
      <Skeleton className="h-16 w-full" />
      <div className="flex gap-2">
        <Skeleton className="h-6 w-16" />
        <Skeleton className="h-6 w-16" />
        <Skeleton className="h-6 w-12" />
      </div>
    </div>
  );
}
