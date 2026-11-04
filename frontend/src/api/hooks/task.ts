import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type ControlTaskRequest,
  type CreateTaskRequest,
  type DeleteTaskRequest,
  type GetTaskRequest,
  type ListTaskResponse,
  type Task,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 任务管理
// ==============================

export function useListTasks(
  query: PaginationQuery,
  options?: Omit<UseQueryOptions<ListTaskResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['listTasks', query],
    queryFn: () => apiClient.taskService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListTasks(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listTasks', params],
    queryFn: () => apiClient.taskService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetTask(
  req: GetTaskRequest,
  options?: Omit<UseQueryOptions<Task, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['getTask', req],
    queryFn: () => apiClient.taskService.Get(req),
    ...options,
  });
}

export function useCreateTask(
  options?: UseMutationOptions<Record<string, never>, Error, CreateTaskRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.taskService.Create(data),
    ...options,
  });
}

export function useUpdateTask(
  options?: UseMutationOptions<
    Record<string, never>,
    Error,
    { id: number; values: NonNullable<Parameters<typeof apiClient.taskService.Update>[0]['data']> }
  >,
) {
  return useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number;
      values: NonNullable<Parameters<typeof apiClient.taskService.Update>[0]['data']>;
    }) =>
      apiClient.taskService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteTask(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteTaskRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.taskService.Delete(req),
    ...options,
  });
}

// ==============================
// 任务控制
// ==============================

/** 获取任务类型名称列表 */
export async function fetchListTaskTypeNames() {
  return queryClient.fetchQuery({
    queryKey: ['listTaskTypeNames'],
    queryFn: () => apiClient.taskService.ListTaskTypeName({}),
    retry: 0,
  });
}

/** 控制单个任务 */
export function useControlTask(
  options?: UseMutationOptions<
    Record<string, never>,
    Error,
    { typeName: string; controlType: string }
  >,
) {
  return useMutation({
    mutationFn: ({ typeName, controlType }) =>
      apiClient.taskService.ControlTask({
        typeName,
        controlType: controlType as ControlTaskRequest['controlType'],
      }),
    ...options,
  });
}

/** 启动所有任务 */
export function useStartAllTasks(options?: UseMutationOptions<Record<string, never>, Error, void>) {
  return useMutation({
    mutationFn: () => apiClient.taskService.StartAllTask({}),
    ...options,
  });
}

/** 停止所有任务 */
export function useStopAllTasks(options?: UseMutationOptions<Record<string, never>, Error, void>) {
  return useMutation({
    mutationFn: () => apiClient.taskService.StopAllTask({}),
    ...options,
  });
}

/** 重启所有任务 */
export function useRestartAllTasks(
  options?: UseMutationOptions<
    Awaited<ReturnType<typeof apiClient.taskService.RestartAllTask>>,
    Error,
    void
  >,
) {
  return useMutation({
    mutationFn: () => apiClient.taskService.RestartAllTask({}),
    ...options,
  });
}
