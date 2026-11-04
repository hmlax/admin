import { useMutation, useQueryClient, type UseMutationOptions } from '@tanstack/react-query';
import {
  type CreateNotificationRuleRequest,
  type DeleteNotificationRuleRequest,
  type ListNotificationRuleResponse,
  type NotificationRule,
  type TestDispatchNotificationRequest,
  type TestDispatchNotificationResponse,
  type UpdateNotificationRuleRequest,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 通知路由规则（事件类型 → 渠道）
// ==============================

const LIST_KEY = 'listNotificationRules';

export function fetchListNotificationRules(
  query: PaginationQuery,
): Promise<ListNotificationRuleResponse> {
  return queryClient.fetchQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.notificationRuleService.ListNotificationRule(query.toRawParams()),
    retry: 0,
  });
}

export function useCreateNotificationRule(
  options?: UseMutationOptions<NotificationRule, Error, CreateNotificationRuleRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationRuleService.CreateNotificationRule(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useUpdateNotificationRule(
  options?: UseMutationOptions<Record<string, never>, Error, UpdateNotificationRuleRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationRuleService.UpdateNotificationRule(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useDeleteNotificationRule(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteNotificationRuleRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationRuleService.DeleteNotificationRule(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

/**
 * 测试投递：按这条规则当场走一遍完整投递链。
 *
 * 异步规则回的是 SENDING 而不是 SENT —— 结论落在队列那一趟，页面上要按台账状态如实显示，
 * 不能把它翻译成都市"发送成功"。
 */
export function useTestDispatchNotification(
  options?: UseMutationOptions<
    TestDispatchNotificationResponse,
    Error,
    TestDispatchNotificationRequest
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.notificationRuleService.TestDispatchNotification(req),
    ...options,
  });
}
