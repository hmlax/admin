import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type DeleteScriptRequest,
  type GetScriptRequest,
  type ListHookPointsResponse,
  type ListScriptsResponse,
  type CreateScriptRequest,
  type Script,
  type TestRunScriptRequest,
  type TestRunScriptResponse,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 脚本管理
// ==============================

export function useListScripts(
  query: PaginationQuery,
  options?: UseQueryOptions<ListScriptsResponse, Error>,
) {
  return useQuery({
    queryKey: ['listScripts', query],
    queryFn: () => apiClient.scriptService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListScripts(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listScripts', params],
    queryFn: () => apiClient.scriptService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetScript(
  req: GetScriptRequest,
  options?: UseQueryOptions<Script, Error>,
) {
  return useQuery({
    queryKey: ['getScript', req],
    queryFn: () => apiClient.scriptService.Get(req),
    ...options,
  });
}

export function useCreateScript(
  options?: UseMutationOptions<{}, Error, CreateScriptRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.scriptService.Create(data),
    ...options,
  });
}

export function useUpdateScript(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.scriptService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteScript(
  options?: UseMutationOptions<{}, Error, DeleteScriptRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptService.Delete(req),
    ...options,
  });
}

export function useTestRunScript(
  options?: UseMutationOptions<TestRunScriptResponse, Error, TestRunScriptRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptService.TestRun(req),
    ...options,
  });
}

export function useListHookPoints(
  options?: Omit<
    UseQueryOptions<ListHookPointsResponse, Error>,
    'queryKey' | 'queryFn'
  >,
) {
  return useQuery({
    queryKey: ['listHookPoints'],
    queryFn: () => apiClient.scriptService.ListHookPoints({}),
    ...options,
  });
}
