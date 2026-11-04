import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type ListInternalMessageResponse,
  type InternalMessage,
  type GetInternalMessageRequest,
  type DeleteInternalMessageRequest,
  type SendMessageRequest,
  type SendMessageResponse,
  type RevokeMessageRequest,
  type ListInternalMessageCategoryResponse,
  type InternalMessageCategory,
  type GetInternalMessageCategoryRequest,
  type CreateInternalMessageCategoryRequest,
  type DeleteInternalMessageCategoryRequest,
  type ListUserInboxResponse,
  type DeleteNotificationFromInboxRequest,
  type MarkNotificationAsReadRequest,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 内部消息管理
// ==============================
export function useListInternalMessages(
  query: PaginationQuery,
  options?: Omit<UseQueryOptions<ListInternalMessageResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['listInternalMessages', query],
    queryFn: () => apiClient.internalMessageService.ListMessage(query.toRawParams()),
    ...options,
  });
}

export async function fetchListInternalMessages(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listInternalMessages', params],
    queryFn: () => apiClient.internalMessageService.ListMessage(params.toRawParams()),
    retry: 0,
  });
}

export function useGetInternalMessage(
  req: GetInternalMessageRequest,
  // queryKey/queryFn 由本 hook 固定构造，调用方只传行为选项（如 enabled）
  options?: Omit<UseQueryOptions<InternalMessage, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['getInternalMessage', req],
    queryFn: () => apiClient.internalMessageService.GetMessage(req),
    ...options,
  });
}

export function useUpdateInternalMessage(
  options?: UseMutationOptions<
    Record<string, never>,
    Error,
    {
      id: number;
      values: NonNullable<
        Parameters<typeof apiClient.internalMessageService.UpdateMessage>[0]['data']
      >;
    }
  >,
) {
  return useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number;
      values: NonNullable<
        Parameters<typeof apiClient.internalMessageService.UpdateMessage>[0]['data']
      >;
    }) =>
      apiClient.internalMessageService.UpdateMessage({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteInternalMessage(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteInternalMessageRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.internalMessageService.DeleteMessage(data),
    ...options,
  });
}

export function useSendMessage(
  options?: UseMutationOptions<SendMessageResponse, Error, SendMessageRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.internalMessageService.SendMessage(data),
    ...options,
  });
}

export function useRevokeMessage(
  options?: UseMutationOptions<Record<string, never>, Error, RevokeMessageRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.internalMessageService.RevokeMessage(data),
    ...options,
  });
}

// ==============================
// 消息分类管理
// ==============================
export function useListMessageCategories(
  query: PaginationQuery,
  options?: Omit<
    UseQueryOptions<ListInternalMessageCategoryResponse, Error>,
    'queryKey' | 'queryFn'
  >,
) {
  return useQuery({
    queryKey: ['listMessageCategories', query],
    queryFn: () => apiClient.internalMessageCategoryService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListMessageCategories(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listMessageCategories', params],
    queryFn: () => apiClient.internalMessageCategoryService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetMessageCategory(
  req: GetInternalMessageCategoryRequest,
  options?: Omit<UseQueryOptions<InternalMessageCategory, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['getMessageCategory', req],
    queryFn: () => apiClient.internalMessageCategoryService.Get(req),
    ...options,
  });
}

export function useCreateMessageCategory(
  options?: UseMutationOptions<Record<string, never>, Error, CreateInternalMessageCategoryRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.internalMessageCategoryService.Create(data),
    ...options,
  });
}

export function useUpdateMessageCategory(
  options?: UseMutationOptions<
    Record<string, never>,
    Error,
    {
      id: number;
      values: NonNullable<
        Parameters<typeof apiClient.internalMessageCategoryService.Update>[0]['data']
      >;
    }
  >,
) {
  return useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number;
      values: NonNullable<
        Parameters<typeof apiClient.internalMessageCategoryService.Update>[0]['data']
      >;
    }) =>
      apiClient.internalMessageCategoryService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteMessageCategory(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteInternalMessageCategoryRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.internalMessageCategoryService.Delete(data),
    ...options,
  });
}

// ==============================
// 消息接收者管理（用户收件箱）
// ==============================
export function useListUserInbox(
  query: PaginationQuery,
  options?: Omit<UseQueryOptions<ListUserInboxResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['listUserInbox', query],
    queryFn: () => apiClient.internalMessageRecipientService.ListUserInbox(query.toRawParams()),
    ...options,
  });
}

export async function fetchListUserInbox(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listUserInbox', params],
    queryFn: () => apiClient.internalMessageRecipientService.ListUserInbox(params.toRawParams()),
    retry: 0,
  });
}

export function useDeleteNotificationFromInbox(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteNotificationFromInboxRequest>,
) {
  return useMutation({
    mutationFn: (data) =>
      apiClient.internalMessageRecipientService.DeleteNotificationFromInbox(data),
    ...options,
  });
}

export function useMarkNotificationAsRead(
  options?: UseMutationOptions<Record<string, never>, Error, MarkNotificationAsReadRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.internalMessageRecipientService.MarkNotificationAsRead(data),
    ...options,
  });
}
