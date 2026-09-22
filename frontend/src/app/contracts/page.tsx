"use client";

import React, { useState, useMemo, useCallback } from "react";
import Link from "next/link";
import {
  Search,
  ArrowRight,
  AlertCircle,
  RotateCcw,
  Calendar,
  GraduationCap,
  Building2,
  CheckCircle2,
  FileCheck,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { listContracts } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { formatJobDate, formatJobBudget, getContractStatusBadgeClasses } from "@/lib/formatters";
import type { ContractStatus, ContractWithDetails } from "@/types/api";

type FilterStatus = "all" | ContractStatus;

const FILTER_TABS: { label: string; value: FilterStatus }[] = [
  { label: "All", value: "all" },
  { label: "Active", value: "active" },
  { label: "Completed", value: "completed" },
  { label: "Draft", value: "draft" },
  { label: "Cancelled", value: "cancelled" },
];

export default function ContractsPage() {
  const { user, backendUser, isAuthenticated, isLoading: isAuthLoading, login } = useAuth();

  const [activeFilter, setActiveFilter] = useState<FilterStatus>("all");
  const [searchQuery, setSearchQuery] = useState("");

  const {
    data: contracts,
    isLoading: isContractsLoading,
    error,
    refetch,
  } = useQuery<ContractWithDetails[]>(
    useCallback((client) => listContracts(client), []),
    { enabled: Boolean(isAuthenticated) }
  );

  const contractsList = useMemo(() => contracts ?? [], [contracts]);

  const counts = useMemo(
    () => ({
      all: contractsList.length,
      active: contractsList.filter((c) => c.status === "active").length,
      completed: contractsList.filter((c) => c.status === "completed").length,
      cancelled: contractsList.filter((c) => c.status === "cancelled").length,
      draft: contractsList.filter((c) => c.status === "draft").length,
    }),
    [contractsList]
  );

  const filteredContracts = useMemo(() => {
    return contractsList.filter((c) => {
      if (activeFilter !== "all" && c.status !== activeFilter) return false;
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase().trim();
        const titleMatch = c.job?.title?.toLowerCase().includes(q) ?? false;
        const companyMatch = (c.client?.company_or_org ?? c.employer?.company_or_org ?? "")
          .toLowerCase()
          .includes(q);
        const clientNameMatch = (c.client?.contact_name ?? c.employer?.contact_name ?? "")
          .toLowerCase()
          .includes(q);
        const freelancerNameMatch =
          `${c.freelancer?.first_name ?? c.student?.first_name ?? ""} ${c.freelancer?.last_name ?? c.student?.last_name ?? ""}`
            .toLowerCase()
            .includes(q);
        const deptMatch = (c.freelancer?.department ?? c.student?.department ?? "")
          .toLowerCase()
          .includes(q);
        return titleMatch || companyMatch || clientNameMatch || freelancerNameMatch || deptMatch;
      }
      return true;
    });
  }, [contractsList, activeFilter, searchQuery]);

  // Unauthenticated state
  if (!isAuthLoading && !isAuthenticated) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-sm rounded-xl border border-border bg-card p-8 text-center">
          <FileCheck className="mx-auto h-8 w-8 text-muted-foreground" />
          <h2 className="mt-4 font-serif text-xl font-medium text-foreground">Sign In Required</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please sign in to view and manage your contracts.
          </p>
          <button
            onClick={() => login({ redirectPath: "/contracts" })}
            className="mt-6 inline-flex w-full items-center justify-center rounded-md bg-foreground px-4 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
          >
            Sign In to Continue
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Page Header */}
      <div className="mb-8">
        <h1 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
          Contracts &amp; Deliverables
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Track active work, complete deliverables, and exchange peer reviews.
        </p>
      </div>

      {/* Filter Strip + Search */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between mb-6 border-b border-border">
        {/* Underline tab strip */}
        <div className="flex items-end gap-0 -mb-px">
          {FILTER_TABS.map((tab) => {
            const count = counts[tab.value];
            const isActive = activeFilter === tab.value;
            return (
              <button
                key={tab.value}
                onClick={() => setActiveFilter(tab.value)}
                className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                  isActive
                    ? "border-foreground text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground hover:border-foreground/30"
                }`}
              >
                {tab.label}
                <span
                  className={`ml-1.5 font-mono text-xs ${
                    isActive ? "text-foreground" : "text-muted-foreground"
                  }`}
                >
                  ({count})
                </span>
              </button>
            );
          })}
        </div>

        {/* Search */}
        <div className="relative w-full sm:w-64 pb-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search contracts..."
            className="w-full rounded-md border border-border bg-card py-1.5 pl-8 pr-3 text-xs text-foreground placeholder:text-muted-foreground focus:border-foreground/50 focus:outline-none transition"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery("")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground hover:text-foreground"
            >
              Clear
            </button>
          )}
        </div>
      </div>

      {/* Error state */}
      {error && (
        <div className="rounded-xl border border-border bg-card p-6 flex items-start gap-3 text-sm text-foreground">
          <AlertCircle className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
          <div className="flex-1">
            <p className="font-medium">Failed to load contracts</p>
            <p className="mt-0.5 text-xs text-muted-foreground">
              {error.message || "An error occurred while fetching your contracts."}
            </p>
          </div>
          <button
            onClick={() => refetch()}
            className="inline-flex items-center gap-1.5 rounded-md border border-border px-3 py-1.5 text-xs font-medium text-foreground transition hover:bg-muted"
          >
            <RotateCcw className="h-3 w-3" />
            Retry
          </button>
        </div>
      )}

      {/* Loading Skeletons */}
      {isContractsLoading && (
        <div className="space-y-3">
          {[1, 2, 3].map((n) => (
            <div
              key={n}
              className="animate-pulse rounded-xl border border-border bg-card p-6 shadow-none"
            >
              <div className="flex flex-col md:flex-row justify-between gap-4">
                <div className="flex-1 space-y-3">
                  <div className="h-3 w-20 rounded bg-muted" />
                  <div className="h-5 w-2/3 rounded bg-muted" />
                  <div className="flex gap-4">
                    <div className="h-3 w-28 rounded bg-muted" />
                    <div className="h-3 w-24 rounded bg-muted" />
                  </div>
                </div>
                <div className="h-8 w-28 rounded bg-muted" />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Empty: no contracts at all */}
      {!isContractsLoading && !error && contractsList.length === 0 && (
        <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none">
          <FileCheck className="mx-auto h-8 w-8 text-muted-foreground" />
          <h3 className="mt-4 font-serif text-lg font-medium text-foreground">No contracts yet</h3>
          <p className="mx-auto mt-2 max-w-sm text-xs text-muted-foreground">
            When an application is accepted, an active contract will appear here to track
            deliverables and peer reviews.
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Link
              href="/jobs"
              className="inline-flex items-center gap-2 rounded-md bg-foreground px-4 py-2 text-xs font-medium text-background transition hover:opacity-90"
            >
              <Search className="h-3.5 w-3.5" />
              Explore Jobs
            </Link>
          </div>
        </div>
      )}

      {/* Empty: filter/search match nothing */}
      {!isContractsLoading &&
        !error &&
        contractsList.length > 0 &&
        filteredContracts.length === 0 && (
          <div className="rounded-xl border border-border bg-card p-10 text-center shadow-none">
            <Search className="mx-auto h-7 w-7 text-muted-foreground" />
            <h3 className="mt-3 font-serif text-base font-medium text-foreground">
              No matching contracts
            </h3>
            <p className="mt-1 text-xs text-muted-foreground">
              No contracts matched &quot;{activeFilter}&quot;
              {searchQuery ? ` and search "${searchQuery}"` : ""}.
            </p>
            <button
              onClick={() => {
                setActiveFilter("all");
                setSearchQuery("");
              }}
              className="mt-4 inline-flex items-center gap-1.5 rounded-md border border-border px-3.5 py-1.5 text-xs font-medium text-foreground transition hover:bg-muted"
            >
              Reset Filters
            </button>
          </div>
        )}

      {/* Contracts List */}
      {!isContractsLoading && !error && filteredContracts.length > 0 && (
        <div className="space-y-3">
          {filteredContracts.map((contract) => {
            const badge = getContractStatusBadgeClasses(contract.status);
            const currentUserId = backendUser?.id || user?.id;
            const freelancerId = contract.freelancer_id;

            const isFreelancer =
              Boolean(currentUserId && freelancerId === currentUserId) ||
              Boolean(
                user?.email &&
                (contract.freelancer?.email === user.email ||
                  contract.student?.email === user.email)
              );

            const counterpartyName = isFreelancer
              ? contract.client?.company_or_org ||
                `${contract.client?.first_name ?? ""} ${contract.client?.last_name ?? ""}`.trim() ||
                contract.employer?.company_or_org ||
                contract.employer?.contact_name ||
                "Client"
              : `${contract.freelancer?.first_name ?? ""} ${contract.freelancer?.last_name ?? ""}`.trim() ||
                `${contract.student?.first_name ?? ""} ${contract.student?.last_name ?? ""}`.trim() ||
                "Freelancer";

            const counterpartyRole = isFreelancer ? "Client" : "Freelancer";

            return (
              <div
                key={contract.id}
                className="rounded-xl border border-border bg-card p-6 shadow-none transition-all hover:border-foreground/30 flex flex-col md:flex-row justify-between gap-4"
              >
                {/* Left: metadata */}
                <div className="flex-1 min-w-0">
                  {/* Contract ID + status pill + budget */}
                  <div className="flex flex-wrap items-center gap-2 mb-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      #{contract.id.slice(0, 8)}
                    </span>
                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider ${badge.bg} ${badge.text}`}
                    >
                      <span className={`h-1.5 w-1.5 rounded-full ${badge.dot}`} />
                      {badge.label}
                    </span>
                    {contract.job && (
                      <span className="font-mono text-xs font-semibold text-foreground ml-auto sm:ml-2">
                        {formatJobBudget(contract.job)}
                      </span>
                    )}
                  </div>

                  {/* Job title */}
                  <Link href={`/contracts/${contract.id}`}>
                    <h2 className="font-serif text-xl font-medium text-foreground hover:underline underline-offset-2 transition-colors">
                      {contract.job?.title || "Contract Agreement"}
                    </h2>
                  </Link>

                  {/* Metadata row */}
                  <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1">
                      {isFreelancer ? (
                        <Building2 className="h-3.5 w-3.5" />
                      ) : (
                        <GraduationCap className="h-3.5 w-3.5" />
                      )}
                      <span>
                        {counterpartyRole}:{" "}
                        <span className="font-medium text-foreground">{counterpartyName}</span>
                      </span>
                    </span>

                    <span className="flex items-center gap-1 font-mono">
                      <Calendar className="h-3.5 w-3.5" />
                      {contract.started_at
                        ? `Started ${formatJobDate(contract.started_at)}`
                        : `Created ${formatJobDate(contract.created_at)}`}
                    </span>

                    {contract.completed_at && (
                      <span className="flex items-center gap-1 text-foreground font-mono">
                        <CheckCircle2 className="h-3.5 w-3.5 text-pastel-greenText" />
                        Completed {formatJobDate(contract.completed_at)}
                      </span>
                    )}
                  </div>
                </div>

                {/* Right: action link */}
                <div className="flex items-center md:items-start md:pt-1">
                  <Link
                    href={`/contracts/${contract.id}`}
                    className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-4 py-2 text-xs font-medium text-foreground shadow-none transition hover:bg-muted/60 active:scale-[0.98] whitespace-nowrap"
                  >
                    {contract.status === "completed"
                      ? "View & Reviews"
                      : contract.status === "active"
                        ? "Manage"
                        : "View Details"}
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
