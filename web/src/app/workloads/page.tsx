"use client";

import { useState } from "react";
import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { useWorkloads } from "@/lib/hooks";
import { WORKLOAD_STATUSES } from "@/lib/types";
import type { WorkloadStatus } from "@/lib/types";

const PAGE_SIZE = 20;

function formatDuration(ms: number | null): string {
  if (ms === null) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatTimestamp(ts: string): string {
  return new Date(ts).toLocaleString();
}

export default function WorkloadsPage() {
  const [offset, setOffset] = useState(0);
  const [statusFilter, setStatusFilter] = useState<WorkloadStatus | "all">(
    "all",
  );

  const { data, isPending } = useWorkloads(PAGE_SIZE, offset);

  const filteredWorkloads =
    data?.workloads.filter(
      (w) => statusFilter === "all" || w.status === statusFilter,
    ) ?? [];

  const total = data?.total ?? 0;
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;
  const totalPages = Math.ceil(total / PAGE_SIZE);

  return (
    <div>
      <PageHeader
        title="Workloads"
        description={`${total} total workloads`}
        action={
          <Link
            href="/workloads/new"
            className="rounded-md bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent/80 transition-colors"
          >
            New Workload
          </Link>
        }
      />

      {/* Status Filter */}
      <div className="flex gap-2 mb-4 flex-wrap">
        <button
          onClick={() => setStatusFilter("all")}
          className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
            statusFilter === "all"
              ? "bg-accent text-white"
              : "bg-muted text-muted-foreground hover:text-foreground"
          }`}
        >
          All
        </button>
        {WORKLOAD_STATUSES.map((status) => (
          <button
            key={status}
            onClick={() => setStatusFilter(status)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium capitalize transition-colors ${
              statusFilter === status
                ? "bg-accent text-white"
                : "bg-muted text-muted-foreground hover:text-foreground"
            }`}
          >
            {status}
          </button>
        ))}
      </div>

      {/* Workload Table */}
      {isPending ? (
        <p className="text-muted-foreground text-sm">Loading workloads...</p>
      ) : filteredWorkloads.length > 0 ? (
        <div className="rounded-lg border border-border overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                  ID
                </th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                  Status
                </th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                  Runtime
                </th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                  Isolation
                </th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                  Created
                </th>
                <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                  Duration
                </th>
              </tr>
            </thead>
            <tbody>
              {filteredWorkloads.map((w) => (
                <tr
                  key={w.id}
                  className="border-b border-border last:border-0 hover:bg-muted/20 transition-colors"
                >
                  <td className="px-4 py-3">
                    <Link
                      href={`/workloads/${w.id}`}
                      className="font-mono text-xs text-accent hover:underline"
                    >
                      {w.id.slice(0, 12)}...
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={w.status} />
                  </td>
                  <td className="px-4 py-3 capitalize">{w.runtime}</td>
                  <td className="px-4 py-3 capitalize">{w.isolation}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {formatTimestamp(w.created_at)}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">
                    {formatDuration(w.duration_ms)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="rounded-lg border border-border bg-muted/30 p-8 text-center">
          <p className="text-muted-foreground">
            {statusFilter === "all"
              ? "No workloads yet."
              : `No ${statusFilter} workloads.`}
          </p>
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <p className="text-sm text-muted-foreground">
            Page {currentPage} of {totalPages}
          </p>
          <div className="flex gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={offset === 0}
              className="rounded-md border border-border px-3 py-1.5 text-sm font-medium transition-colors hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Previous
            </button>
            <button
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={offset + PAGE_SIZE >= total}
              className="rounded-md border border-border px-3 py-1.5 text-sm font-medium transition-colors hover:bg-muted disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
