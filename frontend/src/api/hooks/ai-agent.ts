import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type AiAgentApp,
  type AiAgentRun,
  type CreateAiAgentAppRequest,
  type DeleteAiAgentAppRequest,
  type GetAiAgentRunRequest,
  type ListAiAgentAppsResponse,
  type ListAiAgentRunsResponse,
  type RunAiAgentAppRequest,
  type UpdateAiAgentAppRequest,
} from '@/api/generated/admin/v1';
import { PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// AI Agent 编排（可视化画布）
// ==============================

const LIST_KEY = 'listAiAgentApps';
const RUNS_KEY = 'listAiAgentRuns';

export function useListAiAgentApps(
  query: PaginationQuery,
  options?: Omit<UseQueryOptions<ListAiAgentAppsResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.aiAgentService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListAiAgentApps(query: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.aiAgentService.List(query.toRawParams()),
    retry: 0,
  });
}

export function useCreateAiAgentApp(
  options?: UseMutationOptions<AiAgentApp, Error, CreateAiAgentAppRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.aiAgentService.Create(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useUpdateAiAgentApp(
  options?: UseMutationOptions<Record<string, never>, Error, UpdateAiAgentAppRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.aiAgentService.Update(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useDeleteAiAgentApp(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteAiAgentAppRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.aiAgentService.Delete(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useRunAiAgentApp(
  options?: UseMutationOptions<AiAgentRun, Error, RunAiAgentAppRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.aiAgentService.Run(req),
    ...options,
  });
}

export function useListAiAgentRuns(
  appId: number | undefined,
  query: PaginationQuery,
  options?: Omit<UseQueryOptions<ListAiAgentRunsResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: [RUNS_KEY, appId, query],
    // appId 走 formValues → 通用 eq 过滤（camel→snake 归一由后端机制处理）
    queryFn: () =>
      apiClient.aiAgentService.ListRuns(
        new PaginationQuery({
          paging: { page: query.paging?.page, pageSize: query.paging?.pageSize },
          formValues: appId ? { appId: String(appId) } : undefined,
        }).toRawParams(),
      ),
    enabled: !!appId,
    ...options,
  });
}

export function useGetAiAgentRun(
  options?: UseMutationOptions<AiAgentRun, Error, GetAiAgentRunRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.aiAgentService.GetRun(req),
    ...options,
  });
}
