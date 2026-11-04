import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type CreateTenantRequest,
  type CreateTenantWithAdminUserRequest,
  type DeleteTenantRequest,
  type GetTenantRequest,
  type ListTenantResponse,
  type Tenant,
  type TenantUsage,
  type GetTenantUsageRequest,
  type CleanupTenantDataRequest,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery } from '@/core/transport/rest';
import { apiClient } from '@/api/client';
import { queryClient } from '@/core';

// ==============================
// 获取租户列表
// ==============================
export function useListTenants(
  query: PaginationQuery,
  options?: UseQueryOptions<ListTenantResponse, Error>,
) {
  return useQuery({
    queryKey: ['listTenants', query],
    queryFn: () => apiClient.tenantService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListTenants(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listTenants', params],
    queryFn: () => apiClient.tenantService.List(params.toRawParams()),
    retry: 0,
  });
}

// ==============================
// 获取单个租户
// ==============================
export function useGetTenant(
  req: GetTenantRequest,
  options?: UseQueryOptions<Tenant, Error>,
) {
  return useQuery({
    queryKey: ['getTenant', req],
    queryFn: () => apiClient.tenantService.Get(req),
    ...options,
  });
}

// ==============================
// 创建租户
// ==============================
export function useCreateTenant(
  options?: UseMutationOptions<{}, Error, CreateTenantRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.tenantService.Create(data),
    ...options,
  });
}

// ==============================
// 更新租户
// ==============================
export function useUpdateTenant(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.tenantService.Update({
        id,
        data: {
          ...values,
        },
        updateMask: makeUpdateMask(Object.keys(values ?? [])),
      }),
    ...options,
  });
}

// ==============================
// 删除租户
// ==============================
export function useDeleteTenant(
  options?: UseMutationOptions<{}, Error, DeleteTenantRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.tenantService.Delete(req),
    ...options,
  });
}


export function useCreateTenantWithAdminUser(
  options?: UseMutationOptions<{}, Error, CreateTenantWithAdminUserRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.tenantService.CreateTenantWithAdminUser(req),
    ...options,
  });
}

// ==============================
// 租户用量查询
// ==============================
export function useGetTenantUsage(
  req: GetTenantUsageRequest,
  options?: Omit<UseQueryOptions<TenantUsage, Error, TenantUsage, readonly unknown[]>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['getTenantUsage', req],
    queryFn: () => apiClient.tenantService.GetUsage(req),
    ...options,
  });
}

// ==============================
// 清理租户数据（保留租户记录，状态改为 OFF）
// ==============================
export function useCleanupTenantData(
  options?: UseMutationOptions<{}, Error, CleanupTenantDataRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.tenantService.CleanupData(req),
    ...options,
  });
}
