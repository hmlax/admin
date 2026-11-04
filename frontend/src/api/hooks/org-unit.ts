import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type CreateOrgUnitRequest,
  type DeleteOrgUnitRequest,
  type GetOrgUnitRequest,
  type ListOrgUnitResponse,
  type OrgUnit,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 组织架构管理
// ==============================

export function useListOrgUnits(
  query: PaginationQuery,
  options?: UseQueryOptions<ListOrgUnitResponse, Error>,
) {
  return useQuery({
    queryKey: ['listOrgUnits', query],
    queryFn: () => apiClient.orgUnitService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListOrgUnits(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listOrgUnits', params],
    queryFn: () => apiClient.orgUnitService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetOrgUnit(
  req: GetOrgUnitRequest,
  options?: UseQueryOptions<OrgUnit, Error>,
) {
  return useQuery({
    queryKey: ['getOrgUnit', req],
    queryFn: () => apiClient.orgUnitService.Get(req),
    ...options,
  });
}

export function useCreateOrgUnit(
  options?: UseMutationOptions<{}, Error, CreateOrgUnitRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.orgUnitService.Create(data),
    ...options,
  });
}

export function useUpdateOrgUnit(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.orgUnitService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteOrgUnit(
  options?: UseMutationOptions<{}, Error, DeleteOrgUnitRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.orgUnitService.Delete(req),
    ...options,
  });
}
