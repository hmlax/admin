import { useMutation, useQuery, useQueryClient, type UseMutationOptions, type UseQueryOptions } from '@tanstack/react-query';
import {
  type CreateNotificationChannelRequest,
  type DeleteNotificationChannelRequest,
  type ListNotificationChannelResponse,
  type NotificationChannel,
  type SendTestEmailRequest,
  type SendTestRequest,
  type UpdateNotificationChannelRequest,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 通知渠道（平台级配置）
// ==============================

const LIST_KEY = 'listNotificationChannels';

export function useListNotificationChannels(
  query: PaginationQuery,
  options?: UseQueryOptions<ListNotificationChannelResponse, Error>,
) {
  return useQuery({
    queryKey: ['listNotificationChannels', query],
    queryFn: () => apiClient.notificationChannelService.ListNotificationChannel(query.toRawParams()),
    ...options,
  });
}

export async function fetchListNotificationChannels(query: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.notificationChannelService.ListNotificationChannel(query.toRawParams()),
    retry: 0,
  });
}

export function useCreateNotificationChannel(
  options?: UseMutationOptions<
    NotificationChannel,
    Error,
    CreateNotificationChannelRequest
  >,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationChannelService.CreateNotificationChannel(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useUpdateNotificationChannel(
  options?: UseMutationOptions<{}, Error, UpdateNotificationChannelRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationChannelService.UpdateNotificationChannel(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useDeleteNotificationChannel(
  options?: UseMutationOptions<{}, Error, DeleteNotificationChannelRequest>,
) {
  const queryClientRef = useQueryClient();
  return useMutation({
    mutationFn: (req) => apiClient.notificationChannelService.DeleteNotificationChannel(req),
    onSuccess: () => queryClientRef.invalidateQueries({ queryKey: [LIST_KEY] }),
    ...options,
  });
}

export function useSendTestEmail(
  options?: UseMutationOptions<{}, Error, SendTestEmailRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.notificationChannelService.SendTestEmail(req),
    ...options,
  });
}

export function useSendTest(
  options?: UseMutationOptions<{}, Error, SendTestRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.notificationChannelService.SendTest(req),
    ...options,
  });
}
