import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./vitest.setup.ts'],
    exclude: ['node_modules/**', 'e2e/**'],
    include: [
      'hooks/**/__tests__/**/*.test.ts',
      'hooks/**/__tests__/**/*.test.tsx',
      'components/**/__tests__/**/*.test.tsx',
      'app/**/__tests__/**/*.test.ts',
      'app/**/__tests__/**/*.test.tsx',
      'lib/**/*.test.ts',
      'tests/**/*.test.ts',
    ],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html', 'lcov'],
      exclude: [
        'node_modules/',
        '.next/',
        '.storybook/',
        'coverage/',
        '**/*.d.ts',
        '**/*.config.*',
        '**/dist/**',
        'e2e/**',
      ],
      include: ['hooks/**/*.ts', 'lib/**/*.ts', 'components/**/*.tsx'],
      thresholds: {
        lines: 80,
        functions: 80,
        branches: 80,
        statements: 80,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './'),
    },
  },
});
