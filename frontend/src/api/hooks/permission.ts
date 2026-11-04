import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type Permission,
  type ListPermissionResponse,
  type GetPermissionRequest,
  type CreatePermissionRequest,
  type DeletePermissionRequest,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery } from '@/core/transport/rest';
import { queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 权限点管理
// ==============================

export function useListPermissions(
  query: PaginationQuery,
  options?: UseQueryOptions<ListPermissionResponse, Error>,
) {
  return useQuery({
    queryKey: ['listPermissions', query],
    queryFn: () => apiClient.permissionService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListPermissions(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listPermissions', params],
    queryFn: () => apiClient.permissionService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetPermission(
  req: GetPermissionRequest,
  options?: UseQueryOptions<Permission, Error>,
) {
  return useQuery({
    queryKey: ['getPermission', req],
    queryFn: () => apiClient.permissionService.Get(req),
    ...options,
  });
}

export function useCreatePermission(
  options?: UseMutationOptions<{}, Error, CreatePermissionRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.permissionService.Create(data),
    ...options,
  });
}

export function useUpdatePermission(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.permissionService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeletePermission(
  options?: UseMutationOptions<{}, Error, DeletePermissionRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.permissionService.Delete(req),
    ...options,
  });
}

// ==============================
// 同步权限点（按菜单树重建业务权限分组与权限点）
// ==============================

export function useSyncPermissions(options?: UseMutationOptions<{}, Error>) {
  return useMutation({
    mutationFn: () => apiClient.permissionService.SyncPermissions({}),
    ...options,
  });
}
