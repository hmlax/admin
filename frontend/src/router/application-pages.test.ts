import { describe, expect, it } from 'vitest';
import { pageModules as platformPages } from '../../platform-admin/pages';
import { pageModules as merchantPages } from '../../merchant-admin/pages';
import { businessRoutes as merchantRoutes } from '../../merchant-admin/routes';

describe('application page boundaries', () => {
  it.each([
    'tenant/plan/index.tsx',
    'tenant/tenant/index.tsx',
    'permission/menu/index.tsx',
    'permission/api/index.tsx',
    'permission/permission/index.tsx',
    'notification/channel/index.tsx',
    'notification/template/index.tsx',
    'system/config/index.tsx',
    'system/server-monitor/index.tsx',
  ])('includes %s only in the platform build', (page) => {
    const key = `../src/pages/app/${page}`;
    expect(platformPages).toHaveProperty(key);
    expect(Object.keys(merchantPages)).not.toContain(key);
  });

  it.each([
    'dashboard/analytics/index.tsx',
    'permission/role/index.tsx',
    'opm/user/list/index.tsx',
    'opm/user/profile/index.tsx',
    'internal-message/inbox/index.tsx',
    'system/file/index.tsx',
    'ai/chat/index.tsx',
  ])('retains %s for authorized merchant menus', (page) => {
    expect(Object.keys(merchantPages)).toContain(`../src/pages/app/${page}`);
  });

  it('does not install static merchant business menus before server authorization', () => {
    expect(merchantRoutes).toEqual([]);
  });
});
