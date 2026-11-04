import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@app': fileURLToPath(new URL('./platform-admin', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    include: [
      'src/router/**/*.test.ts',
      'src/core/preferences/**/*.test.ts',
      'src/core/access/**/*.test.ts',
      'build/**/*.test.ts',
      'src/config/**/*.test.ts',
      'src/hooks/**/*.test.ts',
    ],
  },
});
