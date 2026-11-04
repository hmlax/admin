import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import { apiClient } from '@/api/client';
import { type PaginationQuery, queryClient } from '@/core';
import { makeUpdateMask } from '@/core/transport/rest/utils';
import type {
  AiDoc as OperatorAiDoc,
  AiKnowledgeBase as OperatorAiKnowledgeBase,
  CreateAiKnowledgeBaseRequest,
  DeleteAiKnowledgeBaseRequest,
  ListAiKnowledgeBaseResponse,
} from '@/api/generated/admin/v1';

// ── 知识库 ──────────────────────────────────────────────────────────

// useListAiKnowledgeBases
export function useListAiKnowledgeBases(
  query: PaginationQuery,
  options?: Omit<UseQueryOptions<ListAiKnowledgeBaseResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['listAiKnowledgeBases', query],
    queryFn: () => apiClient.aiKnowledgeBaseService.List(query.toRawParams()),
    ...options,
  });
}

// fetchListAiKnowledgeBases — 非组件上下文（聊天页选择器下拉）
export async function fetchListAiKnowledgeBases(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listAiKnowledgeBases', params],
    queryFn: () => apiClient.aiKnowledgeBaseService.List(params.toRawParams()),
    retry: 0,
  });
}

// useCreateAiKnowledgeBase
export function useCreateAiKnowledgeBase(
  options?: UseMutationOptions<Record<string, never>, Error, CreateAiKnowledgeBaseRequest>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.aiKnowledgeBaseService.Create(data),
    ...options,
  });
}

// useUpdateAiKnowledgeBase — 固定签名 { id, values }
export function useUpdateAiKnowledgeBase(
  options?: UseMutationOptions<
    Record<string, never>,
    Error,
    {
      id: number;
      values: NonNullable<Parameters<typeof apiClient.aiKnowledgeBaseService.Update>[0]['data']>;
    }
  >,
) {
  return useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number;
      values: NonNullable<Parameters<typeof apiClient.aiKnowledgeBaseService.Update>[0]['data']>;
    }) =>
      apiClient.aiKnowledgeBaseService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

// useDeleteAiKnowledgeBase — 后端级联删除文档与切片
export function useDeleteAiKnowledgeBase(
  options?: UseMutationOptions<Record<string, never>, Error, DeleteAiKnowledgeBaseRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.aiKnowledgeBaseService.Delete(req),
    ...options,
  });
}

// ── 文档 ────────────────────────────────────────────────────────────

// fetchListAiDocs — 文档管理抽屉
export async function fetchListAiDocs(baseId: number) {
  return queryClient.fetchQuery({
    queryKey: ['listAiDocs', baseId],
    queryFn: () => apiClient.aiKnowledgeBaseService.ListDocs({ baseId }),
    retry: 0,
  });
}

// useUploadAiDoc — 上传纯文本：切片 → 向量化 → 落库
export function useUploadAiDoc(
  options?: UseMutationOptions<
    { doc?: OperatorAiDoc; chunkCount?: number },
    Error,
    { baseId: number; name: string; content: string }
  >,
) {
  return useMutation({
    mutationFn: ({ baseId, name, content }) =>
      apiClient.aiKnowledgeBaseService.UploadDoc({ baseId, name, content }),
    ...options,
  });
}

// useUploadDocFile — 文件上传（txt/md/docx/pdf：后端抽取文本后走同一入库链）
// contentBase64 在 protojson 下就是 base64 字符串（bytes 类型）
export function useUploadDocFile(
  options?: UseMutationOptions<
    { doc?: OperatorAiDoc; chunkCount?: number },
    Error,
    { baseId: number; fileName: string; contentBase64: string }
  >,
) {
  return useMutation({
    mutationFn: ({ baseId, fileName, contentBase64 }) =>
      apiClient.aiKnowledgeBaseService.UploadDocFile({ baseId, fileName, contentBase64 }),
    ...options,
  });
}

// useDeleteAiDoc — 级联删除切片
export function useDeleteAiDoc(
  options?: UseMutationOptions<Record<string, never>, Error, { baseId: number; id: number }>,
) {
  return useMutation({
    mutationFn: ({ baseId, id }) => apiClient.aiKnowledgeBaseService.DeleteDoc({ baseId, id }),
    ...options,
  });
}

export type AiKnowledgeBase = OperatorAiKnowledgeBase;
export type AiDoc = OperatorAiDoc;
