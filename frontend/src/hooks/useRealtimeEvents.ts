"use client";

import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/components/auth/AuthProvider";
import { DEFAULT_API_BASE_URL } from "@/lib/api";

/**
 * useRealtimeEvents subscribes to the backend SSE stream and invalidates
 * TanStack Query caches when server-pushed events arrive.
 *
 * Mount this hook once at the top of the authenticated shell so that all
 * pages share a single EventSource connection per session.
 */
export function useRealtimeEvents() {
  const { isAuthenticated } = useAuth();
  let queryClient: ReturnType<typeof useQueryClient> | null = null;
  try {
    queryClient = useQueryClient();
  } catch {
    queryClient = null;
  }

  useEffect(() => {
    if (!isAuthenticated || !queryClient) return;
    if (typeof window === "undefined" || typeof EventSource === "undefined") return;

    const streamUrl = `${DEFAULT_API_BASE_URL}/events/stream`;
    const eventSource = new EventSource(streamUrl, {
      withCredentials: true,
    });

    const invalidateCaches = () => {
      queryClient?.invalidateQueries({ queryKey: ["activity"] });
      queryClient?.invalidateQueries({ queryKey: ["jobs"] });
      queryClient?.invalidateQueries({ queryKey: ["applications"] });
      queryClient?.invalidateQueries({ queryKey: ["contracts"] });
    };

    eventSource.addEventListener("application_updated", invalidateCaches);
    eventSource.addEventListener("contract_status_changed", invalidateCaches);
    eventSource.addEventListener("proposal_accepted", invalidateCaches);

    eventSource.onerror = () => {
      // EventSource auto-reconnects on error; no manual retry needed.
    };

    return () => {
      eventSource.close();
    };
  }, [isAuthenticated, queryClient]);
}
