import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type CreateFileRequest,
  type DeleteFileRequest,
  type File,
  type GetFileRequest,
  type ListFileResponse,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery } from '@/core/transport/rest';
import { queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 文件管理
// ==============================

export function useListFiles(
  query: PaginationQuery,
  options?: UseQueryOptions<ListFileResponse, Error>,
) {
  return useQuery({
    queryKey: ['listFiles', query],
    queryFn: () => apiClient.fileService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListFiles(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listFiles', params],
    queryFn: () => apiClient.fileService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetFile(
  req: GetFileRequest,
  options?: UseQueryOptions<File, Error>,
) {
  return useQuery({
    queryKey: ['getFile', req],
    queryFn: () => apiClient.fileService.Get(req),
    ...options,
  });
}

export function useCreateFile(
  options?: UseMutationOptions<{}, Error, CreateFileRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.fileService.Create(data),
    ...options,
  });
}

export function useUpdateFile(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.fileService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteFile(
  options?: UseMutationOptions<{}, Error, DeleteFileRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.fileService.Delete(data),
    ...options,
  });
}
