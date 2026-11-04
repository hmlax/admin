import { useQuery, type UseQueryOptions } from '@tanstack/react-query';
import {
  type PolicyEvaluationLog,
  type GetPolicyEvaluationLogRequest,
  type ListPolicyEvaluationLogResponse,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 策略评估日志管理
// ==============================

export function useListPolicyEvaluationLogs(
  query: PaginationQuery,
  options?: UseQueryOptions<ListPolicyEvaluationLogResponse, Error>,
) {
  return useQuery({
    queryKey: ['listPolicyEvaluationLogs', query],
    queryFn: () => apiClient.policyEvaluationLogService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListPolicyEvaluationLogs(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listPolicyEvaluationLogs', params],
    queryFn: () => apiClient.policyEvaluationLogService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetPolicyEvaluationLog(
  req: GetPolicyEvaluationLogRequest,
  options?: UseQueryOptions<PolicyEvaluationLog, Error>,
) {
  return useQuery({
    queryKey: ['getPolicyEvaluationLog', req],
    queryFn: () => apiClient.policyEvaluationLogService.Get(req),
    ...options,
  });
}
