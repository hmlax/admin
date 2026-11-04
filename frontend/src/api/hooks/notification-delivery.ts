import {
  type ListNotificationDeliveryResponse,
} from '@/api/generated/admin/v1';
import { type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 通知投递台账（只读）
// ==============================

const LIST_KEY = 'listNotificationDeliveries';

export async function fetchListNotificationDeliveries(
  query: PaginationQuery,
): Promise<ListNotificationDeliveryResponse> {
  return queryClient.fetchQuery({
    queryKey: [LIST_KEY, query],
    queryFn: () => apiClient.notificationService.ListNotificationDelivery(query.toRawParams()),
    retry: 0,
  });
}
