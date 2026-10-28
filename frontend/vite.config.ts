import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

function manualChunks(id: string) {
  const normalizedId = id.replaceAll('\\', '/')
  if (!normalizedId.includes('/node_modules/')) return undefined

  if (/\/node_modules\/(?:@vue|vue|vue-router|pinia)\//.test(normalizedId)) {
    return 'vue-core'
  }
  if (normalizedId.includes('/node_modules/element-plus/')) {
    return 'element-plus'
  }
  if (/\/node_modules\/(?:xlsx|xlsx-js-style|cfb|ssf|crc-32|adler-32|codepage|wmf|word)\//.test(normalizedId)) {
    return 'spreadsheet'
  }
  if (normalizedId.includes('/node_modules/axios/')) {
    return 'http-client'
  }
  return 'vendor'
}

export default defineConfig({
  plugins: [vue(), {
    name: 'replica-build-identity',
    generateBundle() {
      const sourceSHA256 = process.env.SOURCE_SHA256 ?? 'development'
      if (sourceSHA256 !== 'development' && !/^[a-f0-9]{64}$/.test(sourceSHA256)) {
        throw new Error('SOURCE_SHA256 must be a SHA-256 digest or development')
      }
      this.emitFile({ type: 'asset', fileName: 'build-info.json',
        source: JSON.stringify({ sourceSHA256, package: 'omnirack-replica-frontend' }) + '\n' })
    },
  }],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5174,
    proxy: {
      '/api': process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:18085',
      '/health': process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:18085',
    },
  },
  build: {
    chunkSizeWarningLimit: 850,
    rollupOptions: {
      output: { manualChunks },
    },
  },
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'jsdom',
    // Element Plus table/dialog integration tests render large component trees.
    maxWorkers: 2,
    testTimeout: 15000,
    setupFiles: ['./src/test/setup.ts'],
    clearMocks: true,
    restoreMocks: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      reportsDirectory: './coverage',
      include: ['src/views/RoomScreenView.vue'],
    },
  },
})
