# @tanstack/react-query v5 — Usage Guide

**Date**: 2026-02-19
**Version**: 5.90.21
**Docs**: https://tanstack.com/query/v5/docs/framework/react/overview

## Installation

```bash
pnpm add @tanstack/react-query
```

## Setup — QueryClientProvider

```tsx
// providers.tsx (client component)
"use client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());
  return (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );
}
```

## useQuery — Data Fetching

```tsx
import { useQuery } from "@tanstack/react-query";

const { data, isPending, isError, error } = useQuery({
  queryKey: ["workloads", { limit, offset }],
  queryFn: () => fetchWorkloads({ limit, offset }),
  refetchInterval: 5000,       // optional polling
  enabled: true,               // conditional fetching
});
```

## useMutation — Create/Update/Delete

```tsx
import { useMutation, useQueryClient } from "@tanstack/react-query";

const queryClient = useQueryClient();
const mutation = useMutation({
  mutationFn: (body: CreateWorkloadRequest) => createWorkload(body),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ["workloads"] });
  },
});

mutation.mutate(payload);
// or: await mutation.mutateAsync(payload);
```

## Key Patterns

- **queryKey**: Array-based key for caching/dedup. Include params: `["workloads", id]`.
- **refetchInterval**: Number (ms) or `false`. Use for polling active workloads.
- **enabled**: Boolean to conditionally run query. Use `enabled: !!id`.
- **select**: Transform data before returning: `select: (data) => data.workloads`.
- **staleTime**: How long data stays fresh (ms). Default 0.
- **placeholderData**: Show previous data while refetching: `placeholderData: keepPreviousData`.
