"use client";

import { useQuery } from "@tanstack/react-query";
import {
  getWorkload,
  listWorkloads,
  getStats,
  getBackends,
} from "./api";
import { TERMINAL_STATUSES } from "./types";

const ACTIVE_POLL_INTERVAL_MS = 2000;

export function useWorkload(id: string) {
  return useQuery({
    queryKey: ["workload", id],
    queryFn: () => getWorkload(id),
    enabled: !!id,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      if (status && TERMINAL_STATUSES.includes(status)) {
        return false;
      }
      return ACTIVE_POLL_INTERVAL_MS;
    },
  });
}

export function useWorkloads(limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["workloads", { limit, offset }],
    queryFn: () => listWorkloads(limit, offset),
  });
}

export function useStats() {
  return useQuery({
    queryKey: ["stats"],
    queryFn: getStats,
  });
}

export function useBackends() {
  return useQuery({
    queryKey: ["backends"],
    queryFn: getBackends,
  });
}
