import { defineConfig, loadEnv } from 'vite';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

import { handleEnv, createProxy, createVitePlugins, buildOptions } from '../index';
import { validateMerchantCode } from '../../src/config/merchant-code';
import { applicationConfigPlugin } from '../plugins/application-config';

// https://vitejs.dev/config/
const frontendRoot = fileURLToPath(new URL('../../', import.meta.url));

export function createAdminConfig(appRoot: string, kind: 'platform' | 'merchant') {
  return defineConfig(({ mode, command }) => {
    const env = loadEnv(mode, appRoot);
    env.VITE_ADMIN_APP = kind;
    const code = env.VITE_MERCHANT_CODE?.trim() ?? '';
    if (kind === 'merchant' && command === 'serve') {
      validateMerchantCode(code);
    }
    const viteEnv = handleEnv(env);
    const { VITE_SERVER_PORT, VITE_PROXY } = viteEnv;

    return {
      root: appRoot,
      envDir: appRoot,
      define: { 'import.meta.env.VITE_ADMIN_APP': JSON.stringify(kind) },
      publicDir: path.resolve(frontendRoot, 'public'),
      base: '/',
      plugins: [...createVitePlugins(), applicationConfigPlugin(code)],
      cacheDir: path.resolve(frontendRoot, 'node_modules/.vite', kind),
      optimizeDeps: {
        // editor-react 产物含 monaco 的 `?worker` 导入，依赖优化器处理不了，必须走源码链路。
        // 被 exclude 的子树其传递依赖同样不预构建，其中两类是裸 CJS、浏览器解析不了具名导出：
        //   - @tiptap/react 从 use-sync-external-store/shim 具名导入 useSyncExternalStore
        //   - lowlight 深引 highlight.js/lib/*（es/*.js 只是转发到 lib/*.js 的 CJS 实现）
        // 显式纳入预构建即可，预构建会把这些 CJS 转成 ESM 并补齐 default/具名导出。
        exclude: ['@go-wind/editor-react'],
        include: [
          // Shared pages and excluded ESM packages use both React JSX runtimes.
          // Prebundle them explicitly so the browser never receives their CJS entry files.
          'react',
          'react-dom/client',
          'react/jsx-runtime',
          'react/jsx-dev-runtime',
          'lowlight',
          'highlight.js',
          'highlight.js/lib/core',
          'use-sync-external-store/shim',
          'use-sync-external-store/shim/index.js',
          'use-sync-external-store/shim/with-selector',
          'use-sync-external-store/shim/with-selector.js',
        ],
      },
      resolve: {
        alias: {
          '@': path.resolve(frontendRoot, 'src'),
          '#': path.resolve(frontendRoot, 'types'),
          '@app': appRoot,
        },
      },
      css: {
        preprocessorOptions: {
          less: {
            javascriptEnabled: true,
            charset: false,
          },
        },
      },
      server: {
        open: false,
        port: VITE_SERVER_PORT,
        strictPort: true,
        // 跨域处理
        proxy: createProxy(VITE_PROXY, kind === 'merchant' ? code : ''),
        fs: { allow: [frontendRoot] },
        // 减少文件监听开销
        watch: {
          ignored: ['**/node_modules/**', '**/.git/**', '**/dist/**'],
          usePolling: false,
        },
        // HMR 优化，减少开发环境切换卡顿
        hmr: {
          overlay: true,
        },
        // 开发态安全响应头。X-Frame-Options/HSTS/CSP 仅在生产 nginx 生效——
        // DENY 会阻断 vue-devtools 等开发期同源 iframe，HSTS/CSP 依赖 HTTPS。
        headers: {
          'X-Content-Type-Options': 'nosniff',
          'Referrer-Policy': 'strict-origin-when-cross-origin',
        },
      },
      preview: { port: kind === 'platform' ? 16888 : 16889, strictPort: true },
      build: { ...buildOptions(), outDir: path.resolve(appRoot, 'dist'), emptyOutDir: true },
    };
  });
}
