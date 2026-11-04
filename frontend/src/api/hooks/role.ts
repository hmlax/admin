import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type CreateRoleRequest,
  type DeleteRoleRequest,
  type GetRoleRequest,
  type ListRoleResponse,
  type Role,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 角色管理
// ==============================

export function useListRoles(
  query: PaginationQuery,
  options?: UseQueryOptions<ListRoleResponse, Error>,
) {
  return useQuery({
    queryKey: ['listRoles', query],
    queryFn: () => apiClient.roleService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListRoles(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listRoles', params],
    queryFn: () => apiClient.roleService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetRole(
  req: GetRoleRequest,
  options?: UseQueryOptions<Role, Error>,
) {
  return useQuery({
    queryKey: ['getRole', req],
    queryFn: () => apiClient.roleService.Get(req),
    ...options,
  });
}

export function useCreateRole(
  options?: UseMutationOptions<{}, Error, CreateRoleRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.roleService.Create(data),
    ...options,
  });
}

export function useUpdateRole(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.roleService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteRole(
  options?: UseMutationOptions<{}, Error, DeleteRoleRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.roleService.Delete(req),
    ...options,
  });
}
