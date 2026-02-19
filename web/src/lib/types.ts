// Workload status constants
export const WORKLOAD_STATUSES = [
  "pending",
  "running",
  "completed",
  "failed",
  "killed",
] as const;
export type WorkloadStatus = (typeof WORKLOAD_STATUSES)[number];

// Isolation mode constants
export const ISOLATION_MODES = [
  "auto",
  "microvm",
  "isolate",
  "gvisor",
] as const;
export type IsolationMode = (typeof ISOLATION_MODES)[number];

// Runtime constants
export const RUNTIMES = ["go", "node", "python", "wasm", "oci"] as const;
export type Runtime = (typeof RUNTIMES)[number];

// Terminal statuses — workloads in these states won't change
export const TERMINAL_STATUSES: readonly WorkloadStatus[] = [
  "completed",
  "failed",
  "killed",
];

export interface Workload {
  id: string;
  status: WorkloadStatus;
  isolation: IsolationMode;
  runtime: Runtime;
  node_id: string;
  input_hash: string;
  output: string | null;
  exit_code: number | null;
  error: string;
  cpu_limit: number | null;
  mem_limit: number | null;
  timeout_s: number | null;
  duration_ms: number | null;
  created_at: string;
  started_at: string | null;
  finished_at: string | null;
}

export interface WorkloadStats {
  total: number;
  by_status: Record<string, number>;
  by_isolation: Record<string, number>;
  avg_duration_ms: number;
}

export interface BackendCapabilities {
  name: string;
  supported_runtimes: string[];
  supported_isolations: string[];
  max_concurrency: number;
}

export interface BackendInfo {
  name: string;
  capabilities: BackendCapabilities;
}

export interface CreateWorkloadRequest {
  runtime: Runtime;
  isolation?: IsolationMode;
  code?: string;
  input?: Record<string, unknown>;
  resources?: {
    cpus?: number;
    mem_mb?: number;
    timeout_s?: number;
  };
}

export interface WorkloadListResponse {
  workloads: Workload[];
  total: number;
  limit: number;
  offset: number;
}

export interface LogHistoryLine {
  seq: number;
  line: string;
  created_at: string;
}

export interface LogHistoryResponse {
  workload_id: string;
  lines: LogHistoryLine[];
}

export interface ApiError {
  error: string;
}
