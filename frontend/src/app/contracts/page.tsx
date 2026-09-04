"use client";

import React, { useState, useMemo, useCallback } from "react";
import Link from "next/link";
import {
  FileCheck,
  Clock,
  CheckCircle2,
  XCircle,
  Search,
  Briefcase,
  GraduationCap,
  ArrowRight,
  ExternalLink,
  ShieldCheck,
  AlertCircle,
  RotateCcw,
  Calendar,
  Layers,
  Sparkles,
  Building2,
  DollarSign,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { listContracts } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import {
  formatBudget,
  formatJobDate,
  getContractStatusBadgeClasses,
} from "@/lib/formatters";
import type { ContractStatus, ContractWithDetails } from "@/types/api";

type FilterStatus = "all" | ContractStatus;

export default function ContractsPage() {
  const {
    user,
    backendUser,
    role,
    isAuthenticated,
    isLoading: isAuthLoading,
    login,
  } = useAuth();

  const [activeFilter, setActiveFilter] = useState<FilterStatus>("all");
  const [searchQuery, setSearchQuery] = useState("");

  // Fetch contracts involving the authenticated caller
  const {
    data: contracts,
    isLoading: isContractsLoading,
    error,
    refetch,
  } = useQuery<ContractWithDetails[]>(
    useCallback((client) => listContracts(client), []),
    {
      enabled: Boolean(isAuthenticated),
    }
  );

  const contractsList = useMemo(() => contracts ?? [], [contracts]);

  // Compute status counts
  const counts = useMemo(() => {
    return {
      all: contractsList.length,
      active: contractsList.filter((c) => c.status === "active").length,
      completed: contractsList.filter((c) => c.status === "completed").length,
      cancelled: contractsList.filter((c) => c.status === "cancelled").length,
      draft: contractsList.filter((c) => c.status === "draft").length,
    };
  }, [contractsList]);

  // Filter & search contracts
  const filteredContracts = useMemo(() => {
    return contractsList.filter((c) => {
      // Status filter
      if (activeFilter !== "all" && c.status !== activeFilter) {
        return false;
      }

      // Search filter (matches job title, employer, or student)
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase().trim();
        const titleMatch = c.job?.title?.toLowerCase().includes(query) ?? false;
        const companyMatch =
          c.employer?.company_or_org?.toLowerCase().includes(query) ?? false;
        const employerNameMatch =
          c.employer?.contact_name?.toLowerCase().includes(query) ?? false;
        const studentNameMatch =
          `${c.student?.first_name ?? ""} ${c.student?.last_name ?? ""}`
            .toLowerCase()
            .includes(query);
        const deptMatch =
          c.student?.department?.toLowerCase().includes(query) ?? false;

        return (
          titleMatch ||
          companyMatch ||
          employerNameMatch ||
          studentNameMatch ||
          deptMatch
        );
      }

      return true;
    });
  }, [contractsList, activeFilter, searchQuery]);

  // Handle unauthenticated state
  if (!isAuthLoading && !isAuthenticated) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-md rounded-[10px] border border-slate-200 bg-white p-8 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
            <FileCheck className="h-6 w-6" />
          </div>
          <h2 className="mt-4 text-xl font-bold text-slate-900 dark:text-white">
            Sign In Required
          </h2>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            Please sign in to view and manage your active and completed contracts.
          </p>
          <div className="mt-6">
            <button
              onClick={() => login({ redirectPath: "/contracts" })}
              className="inline-flex w-full items-center justify-center rounded-[10px] bg-primary text-primary-foreground px-4 py-2.5 text-sm font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
            >
              Sign In to Continue
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Page Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2.5">
            <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
              <FileCheck className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
                Contracts & Agreements
              </h1>
              <p className="text-sm text-slate-500 dark:text-slate-400">
                Track deliverables, complete milestone agreements, and exchange verified peer reviews
              </p>
            </div>
          </div>
        </div>

        {/* Quick action buttons based on user role */}
        <div className="flex items-center gap-3">
          {role === "employer" ? (
            <Link
              href="/employer/jobs"
              className="inline-flex items-center gap-2 rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
            >
              <Briefcase className="h-4 w-4 text-primary dark:text-emerald-500" />
              <span>My Job Postings</span>
            </Link>
          ) : (
            <Link
              href="/jobs"
              className="inline-flex items-center gap-2 rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
            >
              <Search className="h-4 w-4 text-primary dark:text-emerald-500" />
              <span>Explore Jobs</span>
            </Link>
          )}
        </div>
      </div>

      {/* Metric Overview Cards */}
      <div className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div className="rounded-[10px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between text-slate-500 dark:text-slate-400">
            <span className="text-xs font-medium uppercase tracking-wider">Total Contracts</span>
            <Layers className="h-4 w-4 text-slate-400" />
          </div>
          <p className="mt-2 text-3xl font-bold text-slate-900 dark:text-white">
            {isContractsLoading ? "-" : counts.all}
          </p>
          <span className="text-[11px] text-slate-400">All commitments</span>
        </div>

        <div className="rounded-[10px] border border-blue-200/80 bg-blue-50/40 p-5 shadow-sm dark:border-blue-900/40 dark:bg-blue-950/20">
          <div className="flex items-center justify-between text-blue-600 dark:text-blue-400">
            <span className="text-xs font-medium uppercase tracking-wider">Active</span>
            <Clock className="h-4 w-4" />
          </div>
          <p className="mt-2 text-3xl font-bold text-blue-700 dark:text-blue-300">
            {isContractsLoading ? "-" : counts.active}
          </p>
          <span className="text-[11px] text-blue-600/80 dark:text-blue-400/80">In progress</span>
        </div>

        <div className="rounded-[10px] border border-emerald-200/80 bg-emerald-50/40 p-5 shadow-sm dark:border-emerald-900/40 dark:bg-emerald-950/20">
          <div className="flex items-center justify-between text-emerald-600 dark:text-emerald-400">
            <span className="text-xs font-medium uppercase tracking-wider">Completed</span>
            <CheckCircle2 className="h-4 w-4" />
          </div>
          <p className="mt-2 text-3xl font-bold text-emerald-700 dark:text-emerald-300">
            {isContractsLoading ? "-" : counts.completed}
          </p>
          <span className="text-[11px] text-emerald-600/80 dark:text-emerald-400/80">Fulfilled & reviewed</span>
        </div>

        <div className="rounded-[10px] border border-rose-200/80 bg-rose-50/40 p-5 shadow-sm dark:border-rose-900/40 dark:bg-rose-950/20">
          <div className="flex items-center justify-between text-rose-600 dark:text-rose-400">
            <span className="text-xs font-medium uppercase tracking-wider">Cancelled</span>
            <XCircle className="h-4 w-4" />
          </div>
          <p className="mt-2 text-3xl font-bold text-rose-700 dark:text-rose-300">
            {isContractsLoading ? "-" : counts.cancelled}
          </p>
          <span className="text-[11px] text-rose-600/80 dark:text-rose-400/80">Terminated early</span>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <div className="mt-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        {/* Status Filter Tabs */}
        <div className="flex flex-wrap gap-1.5 rounded-[10px] border border-slate-200 bg-slate-100/80 p-1 dark:border-slate-800 dark:bg-slate-900">
          <button
            onClick={() => setActiveFilter("all")}
            className={`rounded-[10px] px-3.5 py-1.5 text-xs font-medium transition ${
              activeFilter === "all"
                ? "bg-white text-slate-900 shadow-sm dark:bg-slate-800 dark:text-white"
                : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            }`}
          >
            All ({counts.all})
          </button>
          <button
            onClick={() => setActiveFilter("active")}
            className={`rounded-[10px] px-3.5 py-1.5 text-xs font-medium transition ${
              activeFilter === "active"
                ? "bg-white text-slate-900 shadow-sm dark:bg-slate-800 dark:text-white"
                : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            }`}
          >
            Active ({counts.active})
          </button>
          <button
            onClick={() => setActiveFilter("completed")}
            className={`rounded-[10px] px-3.5 py-1.5 text-xs font-medium transition ${
              activeFilter === "completed"
                ? "bg-white text-slate-900 shadow-sm dark:bg-slate-800 dark:text-white"
                : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            }`}
          >
            Completed ({counts.completed})
          </button>
          <button
            onClick={() => setActiveFilter("cancelled")}
            className={`rounded-[10px] px-3.5 py-1.5 text-xs font-medium transition ${
              activeFilter === "cancelled"
                ? "bg-white text-slate-900 shadow-sm dark:bg-slate-800 dark:text-white"
                : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            }`}
          >
            Cancelled ({counts.cancelled})
          </button>
        </div>

        {/* Search Input */}
        <div className="relative w-full sm:w-72">
          <Search className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search by job or collaborator..."
            className="w-full rounded-[10px] border border-slate-200 bg-white py-2 pl-9 pr-4 text-xs text-slate-900 placeholder:text-slate-400 focus:border-primary focus:outline-none focus:ring-2 focus:ring-ring/20 dark:border-slate-800 dark:bg-slate-900 dark:text-white dark:placeholder:text-slate-500"
          />
          {searchQuery && (
            <button
              onClick={() => setSearchQuery("")}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
            >
              Clear
            </button>
          )}
        </div>
      </div>

      {/* Error state */}
      {error && (
        <div className="mt-8 rounded-[10px] border border-rose-200 bg-rose-50 p-6 text-center dark:border-rose-900/60 dark:bg-rose-950/40">
          <AlertCircle className="mx-auto h-8 w-8 text-rose-600 dark:text-rose-400" />
          <h3 className="mt-3 text-sm font-semibold text-rose-900 dark:text-rose-200">
            Failed to load contracts
          </h3>
          <p className="mt-1 text-xs text-rose-700 dark:text-rose-300">
            {error.message || "An error occurred while fetching your contracts."}
          </p>
          <button
            onClick={() => refetch()}
            className="mt-4 inline-flex items-center gap-1.5 rounded-[10px] border border-rose-300 bg-white px-3.5 py-1.5 text-xs font-semibold text-rose-700 shadow-sm transition hover:bg-rose-50 dark:border-rose-800 dark:bg-slate-900 dark:text-rose-300 dark:hover:bg-slate-800"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            <span>Try Again</span>
          </button>
        </div>
      )}

      {/* Loading Skeletons */}
      {isContractsLoading && (
        <div className="mt-6 space-y-4">
          {[1, 2, 3].map((n) => (
            <div
              key={n}
              className="animate-pulse rounded-[10px] border border-slate-200 bg-white p-6 dark:border-slate-800 dark:bg-slate-900"
            >
              <div className="flex items-center justify-between">
                <div className="h-4 w-24 rounded bg-slate-200 dark:bg-slate-800" />
                <div className="h-6 w-20 rounded-full bg-slate-200 dark:bg-slate-800" />
              </div>
              <div className="mt-4 h-6 w-2/3 rounded bg-slate-200 dark:bg-slate-800" />
              <div className="mt-4 flex gap-6">
                <div className="h-4 w-32 rounded bg-slate-200 dark:bg-slate-800" />
                <div className="h-4 w-28 rounded bg-slate-200 dark:bg-slate-800" />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Empty State: No contracts at all */}
      {!isContractsLoading && !error && contractsList.length === 0 && (
        <div className="mt-8 rounded-[10px] border border-dashed border-slate-300 bg-white p-12 text-center dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
            <FileCheck className="h-7 w-7" />
          </div>
          <h3 className="mt-4 text-base font-semibold text-slate-900 dark:text-white">
            No contracts yet
          </h3>
          <p className="mx-auto mt-2 max-w-md text-xs text-slate-500 dark:text-slate-400">
            {role === "employer"
              ? "When you accept a student's proposal from your job applicant dashboard, an active contract is created automatically."
              : "When an employer accepts your job application, an active contract will appear here to track work and reviews."}
          </p>
          <div className="mt-6">
            {role === "employer" ? (
              <Link
                href="/employer/jobs"
                className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-4 py-2.5 text-xs font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
              >
                <Briefcase className="h-4 w-4" />
                <span>View Job Postings</span>
              </Link>
            ) : (
              <Link
                href="/jobs"
                className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-4 py-2.5 text-xs font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
              >
                <Search className="h-4 w-4" />
                <span>Explore Open Jobs</span>
              </Link>
            )}
          </div>
        </div>
      )}

      {/* Empty State: Filter or Search matches nothing */}
      {!isContractsLoading &&
        !error &&
        contractsList.length > 0 &&
        filteredContracts.length === 0 && (
          <div className="mt-8 rounded-[10px] border border-slate-200 bg-white p-12 text-center dark:border-slate-800 dark:bg-slate-900">
            <Search className="mx-auto h-8 w-8 text-slate-400" />
            <h3 className="mt-3 text-sm font-semibold text-slate-900 dark:text-white">
              No matching contracts found
            </h3>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              No contracts matched the selected status &quot;{activeFilter}&quot;
              {searchQuery ? ` and search &quot;${searchQuery}&quot;` : ""}.
            </p>
            <button
              onClick={() => {
                setActiveFilter("all");
                setSearchQuery("");
              }}
              className="mt-4 inline-flex items-center gap-1.5 rounded-[10px] border border-slate-200 bg-white px-3.5 py-1.5 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
            >
              Reset Filters
            </button>
          </div>
        )}

      {/* Contracts List */}
      {!isContractsLoading && !error && filteredContracts.length > 0 && (
        <div className="mt-6 space-y-4">
          {filteredContracts.map((contract) => {
            const badge = getContractStatusBadgeClasses(contract.status);
            const isStudent =
              contract.student_id === backendUser?.id ||
              contract.student_id === user?.id ||
              role === "student";

            const counterpartyName = isStudent
              ? contract.employer?.company_or_org ||
                contract.employer?.contact_name ||
                "Employer"
              : `${contract.student?.first_name ?? ""} ${
                  contract.student?.last_name ?? ""
                }`.trim() || "Student";

            const counterpartyRole = isStudent ? "Employer" : "Student";

            return (
              <div
                key={contract.id}
                className="group relative overflow-hidden rounded-[10px] border border-slate-200/90 bg-white p-6 shadow-sm transition hover:border-slate-300 hover:shadow-md dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700"
              >
                <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                  {/* Left Column: Job & Collaborator info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex flex-wrap items-center gap-2.5">
                      {/* Status Badge */}
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold ${badge.bg} ${badge.text}`}
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${badge.dot}`} />
                        {badge.label}
                      </span>

                      {/* Contract ID snippet */}
                      <span className="text-[11px] font-mono text-slate-400 dark:text-slate-500">
                        #{contract.id.slice(0, 8)}
                      </span>
                    </div>

                    {/* Job Title */}
                    <h2 className="mt-3 text-lg font-bold text-slate-900 group-hover:text-primary dark:text-white dark:group-hover:text-primary transition">
                      <Link href={`/contracts/${contract.id}`}>
                        {contract.job?.title || "Contract Agreement"}
                      </Link>
                    </h2>

                    {/* Counterparty & Metadata pills */}
                    <div className="mt-3 flex flex-wrap items-center gap-y-2 gap-x-4 text-xs text-slate-600 dark:text-slate-400">
                      {/* Counterparty */}
                      <div className="flex items-center gap-1.5">
                        {isStudent ? (
                          <Building2 className="h-3.5 w-3.5 text-primary dark:text-emerald-500" />
                        ) : (
                          <GraduationCap className="h-3.5 w-3.5 text-primary dark:text-emerald-500" />
                        )}
                        <span>
                          {counterpartyRole}:{" "}
                          <strong className="font-semibold text-slate-900 dark:text-white">
                            {counterpartyName}
                          </strong>
                        </span>
                        {!isStudent && contract.student?.department && (
                          <span className="text-slate-400">
                            • {contract.student.department}
                          </span>
                        )}
                      </div>

                      {/* Agreed Budget */}
                      <div className="flex items-center gap-1.5 font-medium text-slate-900 dark:text-white">
                        <DollarSign className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                        <span>
                          {formatBudget(
                            contract.agreed_budget,
                            contract.job?.pay_type || "fixed"
                          )}
                        </span>
                      </div>

                      {/* Started / Created Date */}
                      <div className="flex items-center gap-1.5 text-slate-500">
                        <Calendar className="h-3.5 w-3.5" />
                        <span>
                          {contract.started_at
                            ? `Started ${formatJobDate(contract.started_at)}`
                            : `Created ${formatJobDate(contract.created_at)}`}
                        </span>
                      </div>

                      {/* Completed Date if fulfilled */}
                      {contract.completed_at && (
                        <div className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 font-medium">
                          <CheckCircle2 className="h-3.5 w-3.5" />
                          <span>Completed {formatJobDate(contract.completed_at)}</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Right Column: Action CTA */}
                  <div className="flex sm:flex-col sm:items-end justify-between items-center pt-2 sm:pt-0 border-t border-slate-100 dark:border-slate-800 sm:border-0">
                    <Link
                      href={`/contracts/${contract.id}`}
                      className="inline-flex items-center gap-1.5 rounded-[10px] bg-slate-100 px-4 py-2 text-xs font-semibold text-slate-800 transition hover:bg-primary text-primary-foreground hover:text-white dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-primary text-primary-foreground dark:hover:text-white"
                    >
                      <span>
                        {contract.status === "completed"
                          ? "View & Reviews"
                          : contract.status === "active"
                          ? "Manage Contract"
                          : "View Details"}
                      </span>
                      <ArrowRight className="h-3.5 w-3.5" />
                    </Link>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
