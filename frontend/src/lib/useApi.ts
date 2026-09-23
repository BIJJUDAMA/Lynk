"use client";

/**
 * Lynk Custom Fetch Hooks
 *
 * Provides declarative React hooks for data fetching and mutations:
 * - useQuery: Declarative querying with automated loading state, error catching, and refetch.
 * - useMutation: Imperative mutation triggers with loading, error tracking, and reset capabilities.
 * - Automatically utilizes getToken from useAuth() to instantiate authenticated ApiClient instances.
 */

import { useState, useEffect, useCallback, useMemo, useRef, DependencyList } from "react";
import { useAuth } from "@/components/auth/AuthProvider";
import { ApiClient, ApiClientError, createApiClient } from "./api";

// ============================================================================
// Query Interfaces & Types
// ============================================================================

export interface UseQueryOptions<T> {
  enabled?: boolean;
  initialData?: T | null;
  deps?: DependencyList;
  onSuccess?: (data: T) => void;
  onError?: (error: ApiClientError | Error) => void;
}

export interface UseQueryResult<T> {
  data: T | null;
  isLoading: boolean;
  error: ApiClientError | Error | null;
  refetch: () => Promise<void>;
}

// ============================================================================
// Mutation Interfaces & Types
// ============================================================================

export interface UseMutationOptions<TArgs, TResult> {
  onSuccess?: (data: TResult, variables: TArgs) => void;
  onError?: (error: ApiClientError | Error, variables: TArgs) => void;
  onSettled?: (
    data: TResult | null,
    error: ApiClientError | Error | null,
    variables: TArgs
  ) => void;
}

export interface UseMutationResult<TArgs, TResult> {
  mutate: (args: TArgs) => Promise<TResult>;
  data: TResult | null;
  isLoading: boolean;
  error: ApiClientError | Error | null;
  reset: () => void;
}

// ============================================================================
// useQuery Hook
// ============================================================================

/**
 * Declarative hook for fetching data via ApiClient with lifecycle management.
 *
 * @param queryFn Function receiving authenticated ApiClient and returning data Promise.
 * @param depsOrOptions Optional dependency list array or options object ({ enabled, initialData, deps, onSuccess, onError }).
 */
export function useQuery<T>(
  queryFn: (client: ApiClient) => Promise<T>,
  depsOrOptions: DependencyList | UseQueryOptions<T> = []
): UseQueryResult<T> {
  const { getToken } = useAuth();
  const client = useMemo(() => createApiClient(getToken), [getToken]);

  const isOptionsObject =
    depsOrOptions !== null && typeof depsOrOptions === "object" && !Array.isArray(depsOrOptions);

  const options: UseQueryOptions<T> = isOptionsObject ? (depsOrOptions as UseQueryOptions<T>) : {};

  const deps: DependencyList = isOptionsObject
    ? (options.deps ?? [])
    : (depsOrOptions as DependencyList);

  const enabled = options.enabled ?? true;
  const initialData = options.initialData ?? null;

  const [data, setData] = useState<T | null>(initialData);
  const [isLoading, setIsLoading] = useState<boolean>(enabled);
  const [error, setError] = useState<ApiClientError | Error | null>(null);

  const queryFnRef = useRef(queryFn);
  const optionsRef = useRef(options);
  const isMountedRef = useRef(true);

  useEffect(() => {
    queryFnRef.current = queryFn;
    optionsRef.current = options;
  });

  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  const execute = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await queryFnRef.current(client);
      if (isMountedRef.current) {
        setData(result);
        optionsRef.current.onSuccess?.(result);
      }
    } catch (err: unknown) {
      if (isMountedRef.current) {
        const apiErr =
          err instanceof ApiClientError || err instanceof Error
            ? err
            : new ApiClientError(String(err) || "Unknown query error", "UNKNOWN_ERROR", 500);
        setError(apiErr);
        optionsRef.current.onError?.(apiErr);
      }
    } finally {
      if (isMountedRef.current) {
        setIsLoading(false);
      }
    }
  }, [client]);

  const refetch = useCallback(async () => {
    await execute();
  }, [execute]);

  useEffect(() => {
    if (!enabled) {
      return;
    }

    setIsLoading(true);
    setError(null);

    queryFnRef
      .current(client)
      .then((result) => {
        if (isMountedRef.current) {
          setData(result);
          optionsRef.current.onSuccess?.(result);
          setIsLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (isMountedRef.current) {
          const apiErr =
            err instanceof ApiClientError || err instanceof Error
              ? err
              : new ApiClientError(String(err) || "Unknown query error", "UNKNOWN_ERROR", 500);
          setError(apiErr);
          optionsRef.current.onError?.(apiErr);
          setIsLoading(false);
        }
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, enabled, ...deps]);

  return {
    data,
    isLoading,
    error,
    refetch,
  };
}

// ============================================================================
// useMutation Hook
// ============================================================================

/**
 * Imperative mutation hook for executing state-changing API operations.
 *
 * @param mutationFn Function accepting mutation arguments and authenticated ApiClient.
 * @param options Optional callbacks for onSuccess, onError, and onSettled.
 */
export function useMutation<TArgs, TResult>(
  mutationFn: (args: TArgs, client: ApiClient) => Promise<TResult>,
  options?: UseMutationOptions<TArgs, TResult>
): UseMutationResult<TArgs, TResult> {
  const { getToken } = useAuth();
  const client = useMemo(() => createApiClient(getToken), [getToken]);

  const [data, setData] = useState<TResult | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [error, setError] = useState<ApiClientError | Error | null>(null);

  const mutationFnRef = useRef(mutationFn);
  const optionsRef = useRef(options);

  useEffect(() => {
    mutationFnRef.current = mutationFn;
    optionsRef.current = options;
  });

  const reset = useCallback(() => {
    setData(null);
    setIsLoading(false);
    setError(null);
  }, []);

  const mutate = useCallback(
    async (args: TArgs): Promise<TResult> => {
      setIsLoading(true);
      setError(null);

      try {
        const result = await mutationFnRef.current(args, client);
        setData(result);
        optionsRef.current?.onSuccess?.(result, args);
        optionsRef.current?.onSettled?.(result, null, args);
        return result;
      } catch (err: unknown) {
        const apiErr =
          err instanceof ApiClientError || err instanceof Error
            ? err
            : new ApiClientError(String(err) || "Unknown mutation error", "UNKNOWN_ERROR", 500);
        setError(apiErr);
        optionsRef.current?.onError?.(apiErr, args);
        optionsRef.current?.onSettled?.(null, apiErr, args);
        throw apiErr;
      } finally {
        setIsLoading(false);
      }
    },
    [client]
  );

  return {
    mutate,
    data,
    isLoading,
    error,
    reset,
  };
}
