import { useMutation, type UseMutationOptions } from '@tanstack/react-query';
import {
  type CreatePrintTemplateRequest,
  type DeletePrintTemplateRequest,
} from '@/api/generated/admin/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 打印模板（单据打印设计器 TemplateData 载体）
// ==============================

export async function fetchListPrintTemplates(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listPrintTemplates', params],
    queryFn: () => apiClient.printTemplateService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCreatePrintTemplate(
  options?: UseMutationOptions<Record<string, never>, Error, CreatePrintTemplateRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.printTemplateService.Create(data),
    ...options,
  });
}

export function useUpdatePrintTemplate(
  options?: UseMutationOptions<
    Record<string, never>,
    Error,
    {
      id: number;
      values: NonNullable<Parameters<typeof apiClient.printTemplateService.Update>[0]['data']>;
    }
  >,
) {
  return useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number;
      values: NonNullable<Parameters<typeof apiClient.printTemplateService.Update>[0]['data']>;
    }) =>
      apiClient.printTemplateService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeletePrintTemplate(
  options?: UseMutationOptions<Record<string, never>, Error, DeletePrintTemplateRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.printTemplateService.Delete(req),
    ...options,
  });
}
