import { defineConfig, devices } from '@playwright/test'

const liveMode = process.env.E2E_LIVE === 'true'
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:4173'
const browserProjectName = `chromium-${process.platform}`
const browserChannel = process.platform === 'win32' ? 'msedge' : 'chromium'

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['list'],
    ['html', { outputFolder: 'playwright-report', open: 'never' }],
  ],
  outputDir: 'test-results',
  snapshotPathTemplate: '{testDir}/snapshots/{testFilePath}/{arg}-{projectName}{ext}',
  expect: {
    toHaveScreenshot: {
      animations: 'disabled',
      caret: 'hide',
      maxDiffPixelRatio: 0.015,
    },
  },
  use: {
    baseURL,
    channel: browserChannel,
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  webServer: liveMode ? undefined : {
    command: 'node ./node_modules/vite/bin/vite.js --host 127.0.0.1 --port 4173',
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    {
      name: 'auth',
      testMatch: /auth\.setup\.ts/,
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: browserProjectName,
      testIgnore: [/auth\.setup\.ts/, /live-smoke\.spec\.ts/, /replica-live-pages\.spec\.ts/],
      dependencies: ['auth'],
      use: {
        ...devices['Desktop Chrome'],
        storageState: '.auth/user.json',
      },
    },
    ...(liveMode ? [{
      name: 'live-smoke',
      testMatch: /(?:live-smoke|replica-live-pages)\.spec\.ts/,
      dependencies: ['auth'],
      use: {
        ...devices['Desktop Chrome'],
        storageState: '.auth/user.json',
      },
    }] : []),
  ],
})
