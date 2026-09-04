"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  ShieldAlert,
  CheckCircle2,
  ArrowRight,
  GraduationCap,
  Loader2,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  clearCodeVerifier,
  clearRedirectPath,
  clearRoleHint,
  exchangeCodeForTokens,
  getStoredCodeVerifier,
  getStoredRedirectPath,
  getStoredRoleHint,
  syncUserWithBackend,
} from "@/lib/auth";
import { User } from "@/types/api";

function CallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setSession } = useAuth();

  const [status, setStatus] = useState<"processing" | "success" | "error">("processing");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Guard against React 18 strict mode double-invoking useEffect
  const processedRef = useRef(false);

  useEffect(() => {
    if (processedRef.current) return;
    processedRef.current = true;

    const handleCallback = async () => {
      const code = searchParams.get("code");
      const error = searchParams.get("error");
      const errorDescription = searchParams.get("error_description");

      if (error) {
        setStatus("error");
        setErrorMessage(
          errorDescription ||
            `Keycloak authorization returned error code: ${error}`
        );
        return;
      }

      if (!code) {
        setStatus("error");
        setErrorMessage("Missing authorization code in Keycloak callback response.");
        return;
      }

      const codeVerifier = getStoredCodeVerifier();
      if (!codeVerifier) {
        setStatus("error");
        setErrorMessage(
          "PKCE code verifier is missing or expired from your session. Please initiate login again."
        );
        return;
      }

      try {
        const redirectUri = `${window.location.origin}/callback`;
        const tokens = await exchangeCodeForTokens(code, codeVerifier, redirectUri);
        clearCodeVerifier();

        const roleHint = getStoredRoleHint() || undefined;
        clearRoleHint();

        // Synchronize authenticated user with Go backend PostgreSQL
        let syncedUser: User | null = null;
        try {
          const syncResponse = await syncUserWithBackend(tokens.accessToken, roleHint);
          if (syncResponse && syncResponse.success && syncResponse.data) {
            syncedUser = syncResponse.data;
          }
        } catch (syncErr) {
          console.warn("User sync with Go backend warning:", syncErr);
        }

        // Initialize session in React AuthProvider
        setSession(tokens, syncedUser);
        setStatus("success");

        const targetPath = getStoredRedirectPath() || "/jobs";
        clearRedirectPath();

        // Brief delay for visual confirmation before routing
        setTimeout(() => {
          router.replace(targetPath);
        }, 600);
      } catch (err: unknown) {
        console.error("Callback token exchange failed:", err);
        setStatus("error");
        setErrorMessage(
          err instanceof Error
            ? err.message
            : "An unexpected error occurred during token exchange."
        );
      }
    };

    handleCallback();
  }, [searchParams, setSession, router]);

  return (
    <div className="flex min-h-[85vh] items-center justify-center px-4 py-12">
      <div className="w-full max-w-md rounded-[10px] border border-slate-200 bg-white p-8 text-center shadow-lg dark:border-slate-800 dark:bg-slate-900">
        {status === "processing" && (
          <div className="space-y-4">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
              <Loader2 className="h-8 w-8 animate-spin" />
            </div>
            <h2 className="text-xl font-bold tracking-tight text-slate-900 dark:text-white">
              Authenticating with Lynk
            </h2>
            <p className="text-sm text-slate-600 dark:text-slate-400">
              Exchanging PKCE cryptographic tokens and verifying campus credentials...
            </p>
          </div>
        )}

        {status === "success" && (
          <div className="space-y-4">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
              <CheckCircle2 className="h-8 w-8" />
            </div>
            <h2 className="text-xl font-bold tracking-tight text-slate-900 dark:text-white">
              Identity Verified!
            </h2>
            <p className="text-sm text-slate-600 dark:text-slate-400">
              Signed in successfully. Redirecting you to the platform...
            </p>
          </div>
        )}

        {status === "error" && (
          <div className="space-y-5">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400">
              <ShieldAlert className="h-8 w-8" />
            </div>
            <div>
              <h2 className="text-xl font-bold tracking-tight text-slate-900 dark:text-white">
                Authentication Failed
              </h2>
              <p className="mt-2 text-xs text-rose-600 dark:text-rose-400">
                {errorMessage || "Unable to complete authorization code exchange."}
              </p>
            </div>

            <div className="pt-2">
              <Link
                href="/login"
                className="inline-flex w-full items-center justify-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-5 py-3 text-sm font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
              >
                <span>Return to Sign In</span>
                <ArrowRight className="h-4 w-4" />
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function CallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[85vh] items-center justify-center">
          <div className="flex flex-col items-center gap-3 text-slate-500">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="text-xs font-medium">Loading authentication context...</p>
          </div>
        </div>
      }
    >
      <CallbackContent />
    </Suspense>
  );
}
