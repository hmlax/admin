import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type DictType,
  type ListDictTypeResponse,
  type ListDictEntryResponse,
  type DeleteDictTypeRequest,
  type CreateDictEntryRequest,
  type DeleteDictEntryRequest,
  type CreateDictTypeRequest,
  type GetDictTypeRequest,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery } from '@/core/transport/rest';
import { queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 字典类型管理
// ==============================

export function useListDictTypes(
  query: PaginationQuery,
  options?: UseQueryOptions<ListDictTypeResponse, Error>,
) {
  return useQuery({
    queryKey: ['listDictTypes', query],
    queryFn: () => apiClient.dictTypeService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListDictTypes(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listDictTypes', params],
    queryFn: () => apiClient.dictTypeService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetDictType(
  req: GetDictTypeRequest,
  options?: UseQueryOptions<DictType, Error>,
) {
  return useQuery({
    queryKey: ['getDictType', req],
    queryFn: () => apiClient.dictTypeService.Get(req),
    ...options,
  });
}

export function useCreateDictType(
  options?: UseMutationOptions<DictType, Error, CreateDictTypeRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.dictTypeService.Create(data),
    ...options,
  });
}

export function useUpdateDictType(
  options?: UseMutationOptions<
    DictType,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.dictTypeService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteDictType(
  options?: UseMutationOptions<{}, Error, DeleteDictTypeRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.dictTypeService.Delete(data),
    ...options,
  });
}

// ==============================
// 字典条目管理
// ==============================

export function useListDictEntries(
  query: PaginationQuery,
  options?: UseQueryOptions<ListDictEntryResponse, Error>,
) {
  return useQuery({
    queryKey: ['listDictEntries', query],
    queryFn: () => apiClient.dictEntryService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListDictEntries(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listDictEntries', params],
    queryFn: () => apiClient.dictEntryService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCreateDictEntry(
  options?: UseMutationOptions<{}, Error, CreateDictEntryRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.dictEntryService.Create(data),
    ...options,
  });
}

export function useUpdateDictEntry(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.dictEntryService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteDictEntry(
  options?: UseMutationOptions<{}, Error, DeleteDictEntryRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.dictEntryService.Delete(data),
    ...options,
  });
}
