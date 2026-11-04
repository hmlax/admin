import { useMutation, useQueryClient, type UseMutationOptions } from '@tanstack/react-query';
import {
  type CreateMonitorAlertRuleRequest,
  type DeleteMonitorAlertRuleRequest,
  type EvaluateMonitorAlertsResponse,
  type ListMonitorAlertRuleResponse,
  type MonitorAlertRule,
  type UpdateMonitorAlertRuleRequest,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 监控告警规则（指标阈值 → 触发通知）
// ==============================

const LIST_KEY = 'listMonitorAlertRules';

export function fetchListMonitorAlertRules(
  query: PaginationQuery,
): Promise<ListMonitorAlertRuleResponse> {
  return queryClient.fetchQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.monitorAlertService.ListMonitorAlertRule(query.toRawParams()),
    retry: 0,
  });
}

export function useCreateMonitorAlertRule(
  options?: UseMutationOptions<
    MonitorAlertRule,
    Error,
    CreateMonitorAlertRuleRequest
  >,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.monitorAlertService.CreateMonitorAlertRule(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useUpdateMonitorAlertRule(
  options?: UseMutationOptions<{}, Error, UpdateMonitorAlertRuleRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.monitorAlertService.UpdateMonitorAlertRule(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useDeleteMonitorAlertRule(
  options?: UseMutationOptions<{}, Error, DeleteMonitorAlertRuleRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.monitorAlertService.DeleteMonitorAlertRule(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useEvaluateMonitorAlerts(
  options?: UseMutationOptions<EvaluateMonitorAlertsResponse, Error, {}>,
) {
  return useMutation({
    mutationFn: () => apiClient.monitorAlertService.EvaluateMonitorAlerts({}),
    ...options,
  });
}
