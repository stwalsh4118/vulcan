import type {
  ApiError,
  CreateWorkloadRequest,
  LogHistoryResponse,
  Workload,
  WorkloadListResponse,
  WorkloadStats,
  BackendInfo,
} from "./types";

const BASE_URL = "/api/v1";

class ApiClientError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiClientError";
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    headers: {
      "Content-Type": "application/json",
    },
    ...options,
  });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const body: ApiError = await response.json();
      message = body.error;
    } catch {
      // Use default message if body isn't JSON
    }
    throw new ApiClientError(response.status, message);
  }

  return response.json() as Promise<T>;
}

export async function getWorkload(id: string): Promise<Workload> {
  return request<Workload>(`/workloads/${id}`);
}

export async function listWorkloads(
  limit = 20,
  offset = 0,
): Promise<WorkloadListResponse> {
  return request<WorkloadListResponse>(
    `/workloads?limit=${limit}&offset=${offset}`,
  );
}

export async function createWorkload(
  req: CreateWorkloadRequest,
): Promise<Workload> {
  return request<Workload>("/workloads/async", {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export async function deleteWorkload(id: string): Promise<Workload> {
  return request<Workload>(`/workloads/${id}`, {
    method: "DELETE",
  });
}

export async function getStats(): Promise<WorkloadStats> {
  return request<WorkloadStats>("/stats");
}

export async function getBackends(): Promise<BackendInfo[]> {
  return request<BackendInfo[]>("/backends");
}

export async function getLogHistory(id: string): Promise<LogHistoryResponse> {
  return request<LogHistoryResponse>(`/workloads/${id}/logs/history`);
}

export { ApiClientError };
