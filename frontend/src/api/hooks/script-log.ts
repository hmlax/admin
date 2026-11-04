import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type ListScriptLogsResponse,
  type PurgeScriptLogsRequest,
  type PurgeScriptLogsResponse,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 脚本执行日志
// ==============================

export function useListScriptLogs(
  query: PaginationQuery,
  options?: UseQueryOptions<ListScriptLogsResponse, Error>,
) {
  return useQuery({
    queryKey: ['listScriptLogs', query],
    queryFn: () => apiClient.scriptLogService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListScriptLogs(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listScriptLogs', params],
    queryFn: () => apiClient.scriptLogService.List(params.toRawParams()),
    retry: 0,
  });
}

export function usePurgeScriptLogs(
  options?: UseMutationOptions<
    PurgeScriptLogsResponse,
    Error,
    PurgeScriptLogsRequest
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptLogService.Purge(req),
    ...options,
  });
}
