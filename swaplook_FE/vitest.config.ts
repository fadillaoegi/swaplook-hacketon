import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.tsx'],
    setupFiles: ['./tests/setup.ts'],
    environmentOptions: { jsdom: { url: 'http://127.0.0.1:5173/' } },
    fileParallelism: false,
  },
})
