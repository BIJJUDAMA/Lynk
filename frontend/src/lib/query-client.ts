import { QueryClient } from "@tanstack/react-query";

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 60 * 1000, // 60 seconds stale-while-revalidate
        gcTime: 10 * 60 * 1000, // 10 minutes cache retention
        retry: 1,
        refetchOnWindowFocus: false,
      },
    },
  });
}

