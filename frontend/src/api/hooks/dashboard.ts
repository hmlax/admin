import type {
  ActionDistributionResponse,
  DashboardOverviewResponse,
  LoginTrendResponse,
  StatusDistributionResponse,
} from '@/api/generated/admin/v1';
import { useQuery, type UseQueryOptions } from '@tanstack/react-query';

import { apiClient } from '@/api/client';

// =====================================================
// 首页分析概览（只读聚合统计）
// =====================================================

// ------------------------------
// 1. 概览统计卡片
// ------------------------------
export function useDashboardOverview(
  options?: Omit<UseQueryOptions<DashboardOverviewResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['dashboardOverview'],
    queryFn: () => apiClient.dashboardService.GetOverview({}),
    ...options,
  });
}

// ------------------------------
// 2. 近 N 天登录趋势
// ------------------------------
export function useLoginTrend(
  days: number,
  options?: Omit<UseQueryOptions<LoginTrendResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['loginTrend', days],
    queryFn: () => apiClient.dashboardService.GetLoginTrend({ days }),
    ...options,
  });
}

// ------------------------------
// 3. 操作审计按 action 分布
// ------------------------------
export function useOperationActionDistribution(
  options?: Omit<UseQueryOptions<ActionDistributionResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['operationActionDistribution'],
    queryFn: () => apiClient.dashboardService.GetOperationActionDistribution({}),
    ...options,
  });
}

// ------------------------------
// 4. 登录审计按 status 分布
// ------------------------------
export function useLoginStatusDistribution(
  options?: Omit<UseQueryOptions<StatusDistributionResponse, Error>, 'queryKey' | 'queryFn'>,
) {
  return useQuery({
    queryKey: ['loginStatusDistribution'],
    queryFn: () => apiClient.dashboardService.GetLoginStatusDistribution({}),
    ...options,
  });
}
