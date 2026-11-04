import { useMutation, useQueryClient, type UseMutationOptions } from '@tanstack/react-query';
import {
  type CreateNotificationTemplateRequest,
  type DeleteNotificationTemplateRequest,
  type ListNotificationTemplateResponse,
  type NotificationTemplate,
  type RenderNotificationTemplateRequest,
  type RenderNotificationTemplateResponse,
  type UpdateNotificationTemplateRequest,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 通知模板（可复用的标题/正文占位模板）
// ==============================

const LIST_KEY = 'listNotificationTemplates';

export function fetchListNotificationTemplates(
  query: PaginationQuery,
): Promise<ListNotificationTemplateResponse> {
  return queryClient.fetchQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.notificationTemplateService.ListNotificationTemplate(query.toRawParams()),
    retry: 0,
  });
}

export function useCreateNotificationTemplate(
  options?: UseMutationOptions<
    NotificationTemplate,
    Error,
    CreateNotificationTemplateRequest
  >,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationTemplateService.CreateNotificationTemplate(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useUpdateNotificationTemplate(
  options?: UseMutationOptions<{}, Error, UpdateNotificationTemplateRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationTemplateService.UpdateNotificationTemplate(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useDeleteNotificationTemplate(
  options?: UseMutationOptions<{}, Error, DeleteNotificationTemplateRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationTemplateService.DeleteNotificationTemplate(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useRenderNotificationTemplate(
  options?: UseMutationOptions<
    RenderNotificationTemplateResponse,
    Error,
    RenderNotificationTemplateRequest
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.notificationTemplateService.RenderNotificationTemplate(req),
    ...options,
  });
}
