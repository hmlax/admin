import { useQuery, type UseQueryOptions } from '@tanstack/react-query';
import {
  type GetPermissionAuditLogRequest,
  type ListPermissionAuditLogResponse,
  type PermissionAuditLog,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 权限审计日志
// ==============================

export function useListPermissionAuditLogs(
  query: PaginationQuery,
  options?: UseQueryOptions<ListPermissionAuditLogResponse, Error>,
) {
  return useQuery({
    queryKey: ['listPermissionAuditLogs', query],
    queryFn: () => apiClient.permissionAuditLogService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListPermissionAuditLogs(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listPermissionAuditLogs', params],
    queryFn: () => apiClient.permissionAuditLogService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetPermissionAuditLog(
  req: GetPermissionAuditLogRequest,
  options?: UseQueryOptions<PermissionAuditLog, Error>,
) {
  return useQuery({
    queryKey: ['getPermissionAuditLog', req],
    queryFn: () => apiClient.permissionAuditLogService.Get(req),
    ...options,
  });
}
