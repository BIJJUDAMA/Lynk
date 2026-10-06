"use client";

import React, { Component, ErrorInfo, ReactNode } from "react";
import { AlertCircle, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";

interface Props {
  children: ReactNode;
  fallbackTitle?: string;
}

interface State {
  hasError: boolean;
  error?: Error;
}

/**
 * ErrorBoundary wraps a subtree and catches runtime render errors,
 * displaying an isolated fallback UI instead of unmounting the whole page.
 *
 * Usage:
 *   <ErrorBoundary fallbackTitle="Jobs temporarily unavailable">
 *     <JobList />
 *   </ErrorBoundary>
 */
export class ErrorBoundary extends Component<Props, State> {
  public state: State = { hasError: false };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("ErrorBoundary caught error:", error, errorInfo);
  }

  public render() {
    if (this.state.hasError) {
      return (
        <div className="rounded-xl border border-destructive/20 bg-destructive/5 p-6 text-center text-xs text-destructive space-y-3">
          <AlertCircle className="mx-auto h-6 w-6" aria-hidden="true" />
          <p className="font-semibold">
            {this.props.fallbackTitle || "Unable to display this section"}
          </p>
          <p className="text-muted-foreground">
            {this.state.error?.message || "An unexpected error occurred."}
          </p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => this.setState({ hasError: false })}
            className="text-xs"
          >
            <RotateCcw className="h-3 w-3 mr-1.5" aria-hidden="true" />
            Try again
          </Button>
        </div>
      );
    }
    return this.props.children;
  }
}
