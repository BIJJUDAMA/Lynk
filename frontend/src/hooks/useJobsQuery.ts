import { useQuery } from "@tanstack/react-query";
import { listJobs } from "@/lib/api";
import type { Job, JobFilter } from "@/types/api";

export function useJobsQuery(filters?: JobFilter) {
  return useQuery<Job[], Error>({
    queryKey: ["jobs", filters],
    queryFn: () => listJobs(filters),
  });
}
