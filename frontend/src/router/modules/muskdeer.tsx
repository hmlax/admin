import type { AppRouteObject } from '@/core/router/types';
import { createLazyRoute } from '@/core/router';
const routes: AppRouteObject[] = [{
 name: 'Muskdeer', path: 'muskdeer', meta: { title: 'routes:Muskdeer', icon: 'lucide:chart-candlestick', order: 20 },
 children: [
{ name: 'MuskdeerOverview', path: 'overview', element: createLazyRoute(() => import('@/pages/app/muskdeer/overview')), meta: { title: 'routes:MuskdeerOverview', authority: ['muskdeer:overview:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerTemplates', path: 'templates', element: createLazyRoute(() => import('@/pages/app/muskdeer/templates')), meta: { title: 'routes:MuskdeerTemplates', authority: ['muskdeer:template:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerInstances', path: 'instances', element: createLazyRoute(() => import('@/pages/app/muskdeer/instances')), meta: { title: 'routes:MuskdeerInstances', authority: ['muskdeer:instance:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerInstruments', path: 'instruments', element: createLazyRoute(() => import('@/pages/app/muskdeer/instruments')), meta: { title: 'routes:MuskdeerInstruments', authority: ['muskdeer:instrument:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerAccounts', path: 'accounts', element: createLazyRoute(() => import('@/pages/app/muskdeer/accounts')), meta: { title: 'routes:MuskdeerAccounts', authority: ['muskdeer:account:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerOrders', path: 'orders', element: createLazyRoute(() => import('@/pages/app/muskdeer/orders')), meta: { title: 'routes:MuskdeerOrders', authority: ['muskdeer:order:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerCancellations', path: 'cancellations', element: createLazyRoute(() => import('@/pages/app/muskdeer/cancellations')), meta: { title: 'routes:MuskdeerCancellations', authority: ['muskdeer:cancellation:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerSynthetic', path: 'synthetic', element: createLazyRoute(() => import('@/pages/app/muskdeer/synthetic')), meta: { title: 'routes:MuskdeerSynthetic', authority: ['muskdeer:synthetic:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerSnapshot', path: 'snapshot', element: createLazyRoute(() => import('@/pages/app/muskdeer/snapshot')), meta: { title: 'routes:MuskdeerSnapshot', authority: ['muskdeer:snapshot:view', 'sys:platform_admin', 'sys:tenant_manager'] } },
{ name: 'MuskdeerBindings', path: 'account-bindings', element: createLazyRoute(() => import('@/pages/app/muskdeer/account-bindings')), meta: { title: 'routes:MuskdeerBindings', authority: ['muskdeer:binding:view', 'sys:platform_admin'] } },
 ]
}];
export default routes;
