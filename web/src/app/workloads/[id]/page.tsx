"use client";

import { use, useState } from "react";
import { PageHeader } from "@/components/ui/PageHeader";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { LogViewer } from "@/components/logs/LogViewer";
import { useWorkload } from "@/lib/hooks";
import { useLogStream } from "@/hooks/useLogStream";
import { deleteWorkload, ApiClientError } from "@/lib/api";
import { TERMINAL_STATUSES } from "@/lib/types";

function formatDuration(ms: number | null): string {
  if (ms === null) return "—";
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatTimestamp(ts: string | null): string {
  if (!ts) return "—";
  return new Date(ts).toLocaleString();
}

function decodeOutput(output: string | null): string | null {
  if (!output) return null;
  try {
    return atob(output);
  } catch {
    // If not base64, return as-is
    return output;
  }
}

export default function WorkloadDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const { data: workload, isPending, isError, error } = useWorkload(id);
  const { lines: logLines, streamState } = useLogStream(id, workload?.status);
  const [killing, setKilling] = useState(false);
  const [killError, setKillError] = useState<string | null>(null);

  async function handleKill() {
    if (!confirm("Are you sure you want to kill this workload?")) return;
    setKilling(true);
    setKillError(null);
    try {
      await deleteWorkload(id);
    } catch (err) {
      if (err instanceof ApiClientError) {
        setKillError(err.message);
      } else {
        setKillError("Failed to kill workload");
      }
    } finally {
      setKilling(false);
    }
  }

  if (isPending) {
    return (
      <div>
        <PageHeader title="Workload Detail" />
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  if (isError) {
    const is404 =
      error instanceof ApiClientError && error.status === 404;
    return (
      <div>
        <PageHeader title="Workload Detail" />
        <div className="rounded-lg border border-border bg-muted/30 p-8 text-center">
          <p className="text-muted-foreground">
            {is404 ? "Workload not found." : "Failed to load workload."}
          </p>
        </div>
      </div>
    );
  }

  const isActive = !TERMINAL_STATUSES.includes(workload.status);

  return (
    <div>
      <PageHeader
        title={`Workload ${workload.id.slice(0, 12)}...`}
        action={
          isActive ? (
            <button
              onClick={handleKill}
              disabled={killing}
              className="rounded-md border border-red-500/30 bg-red-500/10 px-4 py-2 text-sm font-medium text-red-400 hover:bg-red-500/20 transition-colors disabled:opacity-50"
            >
              {killing ? "Killing..." : "Kill"}
            </button>
          ) : undefined
        }
      />

      {killError && (
        <div className="mb-4 rounded-md border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
          {killError}
        </div>
      )}

      {/* Status */}
      <div className="mb-6 flex items-center gap-3">
        <StatusBadge status={workload.status} />
        {isActive && (
          <span className="text-xs text-muted-foreground">
            Auto-refreshing...
          </span>
        )}
      </div>

      {/* Info Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <InfoCard label="Runtime" value={workload.runtime} />
        <InfoCard label="Isolation" value={workload.isolation} />
        <InfoCard label="Exit Code" value={workload.exit_code?.toString() ?? "—"} />
        <InfoCard label="Duration" value={formatDuration(workload.duration_ms)} />
        <InfoCard label="Node ID" value={workload.node_id || "—"} />
        <InfoCard label="Created" value={formatTimestamp(workload.created_at)} />
        <InfoCard label="Started" value={formatTimestamp(workload.started_at)} />
        <InfoCard label="Finished" value={formatTimestamp(workload.finished_at)} />
      </div>

      {/* MicroVM Details */}
      {workload.isolation === "microvm" && (
        <section className="mb-6">
          <h2 className="text-sm font-medium text-muted-foreground mb-2">
            MicroVM Details
          </h2>
          <div className="rounded-lg border border-border bg-muted/30 p-4">
            <div className="flex flex-wrap gap-2">
              <span className="inline-flex items-center rounded-full bg-purple-500/10 border border-purple-500/20 px-3 py-1 text-xs font-medium text-purple-400">
                Firecracker
              </span>
              <span className="inline-flex items-center rounded-full bg-blue-500/10 border border-blue-500/20 px-3 py-1 text-xs font-medium text-blue-400">
                {workload.runtime}
              </span>
            </div>
          </div>
        </section>
      )}

      {/* Output */}
      {workload.output && (
        <section className="mb-6">
          <h2 className="text-sm font-medium text-muted-foreground mb-2">
            Output
          </h2>
          <pre className="rounded-lg border border-border bg-muted/30 p-4 text-sm font-mono overflow-x-auto whitespace-pre-wrap">
            {decodeOutput(workload.output)}
          </pre>
        </section>
      )}

      {/* Error */}
      {workload.error && (
        <section className="mb-6">
          <h2 className="text-sm font-medium text-red-400 mb-2">Error</h2>
          <pre className="rounded-lg border border-red-500/30 bg-red-500/10 p-4 text-sm font-mono text-red-300 overflow-x-auto whitespace-pre-wrap">
            {workload.error}
          </pre>
        </section>
      )}

      {/* Log Streaming */}
      <section>
        <h2 className="text-sm font-medium text-muted-foreground mb-2">
          Logs
        </h2>
        <LogViewer lines={logLines} streamState={streamState} />
      </section>
    </div>
  );
}

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-muted/30 p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="text-sm font-medium capitalize mt-0.5 truncate">{value}</p>
    </div>
  );
}
