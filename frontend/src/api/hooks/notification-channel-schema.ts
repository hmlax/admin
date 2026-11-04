import { useQuery, type UseQueryOptions } from '@tanstack/react-query';
import {
  type ChannelSchema,
  type ListChannelSchemasResponse,
} from '@/api/generated/admin/v1';
import { apiClient } from '@/api/client';

// ==============================
// 扩展渠道 schema（配置表单动态渲染的数据源）
// ==============================

export function useListChannelSchemas(
  options?: UseQueryOptions<ListChannelSchemasResponse, Error>,
) {
  return useQuery({
    queryKey: ['listChannelSchemas'],
    queryFn: () => apiClient.notificationChannelService.ListChannelSchemas({}),
    staleTime: 5 * 60 * 1000,
    ...options,
  });
}

export type { ChannelSchema as ChannelSchema };
