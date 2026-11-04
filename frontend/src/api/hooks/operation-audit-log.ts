import { useQuery, type UseQueryOptions } from '@tanstack/react-query';
import {
  type GetOperationAuditLogRequest,
  type ListOperationAuditLogResponse,
  type OperationAuditLog,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 操作审计日志
// ==============================

export function useListOperationAuditLogs(
  query: PaginationQuery,
  options?: UseQueryOptions<ListOperationAuditLogResponse, Error>,
) {
  return useQuery({
    queryKey: ['listOperationAuditLogs', query],
    queryFn: () => apiClient.operationAuditLogService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListOperationAuditLogs(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listOperationAuditLogs', params],
    queryFn: () => apiClient.operationAuditLogService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetOperationAuditLog(
  req: GetOperationAuditLogRequest,
  options?: UseQueryOptions<OperationAuditLog, Error>,
) {
  return useQuery({
    queryKey: ['getOperationAuditLog', req],
    queryFn: () => apiClient.operationAuditLogService.Get(req),
    ...options,
  });
}
