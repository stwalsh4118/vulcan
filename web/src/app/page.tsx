"use client";

import Link from "next/link";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { useStats, useWorkloads } from "@/lib/hooks";
import type { WorkloadStatus } from "@/lib/types";
import { WORKLOAD_STATUSES, ISOLATION_MODES } from "@/lib/types";

const STATUS_CARD_COLORS: Record<WorkloadStatus, string> = {
  pending: "text-yellow-400",
  running: "text-blue-400",
  completed: "text-green-400",
  failed: "text-red-400",
  killed: "text-zinc-400",
};

function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatTimestamp(ts: string): string {
  return new Date(ts).toLocaleString();
}

export default function DashboardPage() {
  const { data: stats, isPending: statsLoading } = useStats();
  const { data: workloadsData, isPending: workloadsLoading } = useWorkloads(5, 0);

  return (
    <div>
      <PageHeader
        title="Dashboard"
        description="Workload execution overview"
        action={
          <Link
            href="/workloads/new"
            className="rounded-md bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent/80 transition-colors"
          >
            New Workload
          </Link>
        }
      />

      {/* Stats Section */}
      <section className="mb-8">
        <h2 className="text-lg font-semibold mb-4">Overview</h2>
        {statsLoading ? (
          <p className="text-muted-foreground text-sm">Loading stats...</p>
        ) : stats ? (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="rounded-lg border border-border bg-muted/30 p-4">
              <p className="text-sm text-muted-foreground">Total Workloads</p>
              <p className="text-2xl font-bold">{stats.total}</p>
            </div>
            <div className="rounded-lg border border-border bg-muted/30 p-4">
              <p className="text-sm text-muted-foreground">Avg Duration</p>
              <p className="text-2xl font-bold">
                {stats.avg_duration_ms > 0
                  ? formatDuration(stats.avg_duration_ms)
                  : "—"}
              </p>
            </div>
            {WORKLOAD_STATUSES.map((status) => (
              <div
                key={status}
                className="rounded-lg border border-border bg-muted/30 p-4"
              >
                <p className="text-sm text-muted-foreground capitalize">
                  {status}
                </p>
                <p className={`text-2xl font-bold ${STATUS_CARD_COLORS[status]}`}>
                  {stats.by_status?.[status] ?? 0}
                </p>
              </div>
            ))}
            {ISOLATION_MODES.filter((m) => m !== "auto").map((mode) => (
              <div
                key={mode}
                className="rounded-lg border border-border bg-muted/30 p-4"
              >
                <p className="text-sm text-muted-foreground capitalize">
                  {mode}
                </p>
                <p className="text-2xl font-bold">
                  {stats.by_isolation?.[mode] ?? 0}
                </p>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">
            No stats available. Start by submitting a workload.
          </p>
        )}
      </section>

      {/* Recent Workloads Section */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Recent Workloads</h2>
          <Link
            href="/workloads"
            className="text-sm text-accent hover:underline"
          >
            View all
          </Link>
        </div>
        {workloadsLoading ? (
          <p className="text-muted-foreground text-sm">Loading workloads...</p>
        ) : workloadsData && workloadsData.workloads.length > 0 ? (
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
                </tr>
              </thead>
              <tbody>
                {workloadsData.workloads.map((w) => (
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
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="rounded-lg border border-border bg-muted/30 p-8 text-center">
            <p className="text-muted-foreground">No workloads yet.</p>
            <Link
              href="/workloads/new"
              className="mt-2 inline-block text-sm text-accent hover:underline"
            >
              Submit your first workload
            </Link>
          </div>
        )}
      </section>
    </div>
  );
}
