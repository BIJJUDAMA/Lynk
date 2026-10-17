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

/**
 * ProjectCardSkeleton renders an animated placeholder matching a ProjectCard's
 * dimensions for use in the projects discovery grid during data fetching.
 */
export function ProjectCardSkeleton() {
  return (
    <div className="rounded-xl border border-line bg-card p-6 flex flex-col justify-between h-[230px]">
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <Skeleton className="h-5 w-24 rounded-full" />
          <Skeleton className="h-5 w-20 rounded-full" />
          <Skeleton className="h-5 w-16 rounded-full" />
        </div>
        <Skeleton className="h-6 w-4/5" />
        <div className="space-y-1.5 pt-1">
          <Skeleton className="h-3.5 w-full" />
          <Skeleton className="h-3.5 w-2/3" />
        </div>
      </div>
      <div className="pt-4 border-t border-border flex items-center justify-between">
        <div className="space-y-1">
          <Skeleton className="h-3.5 w-24" />
          <Skeleton className="h-3 w-16" />
        </div>
        <Skeleton className="h-4 w-28" />
      </div>
    </div>
  );
}
