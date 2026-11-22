import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// Em dev o gateway (KrakenD) responde em :8000; o nginx do container faz o mesmo proxy em produção local.
const gateway = process.env.VITE_GATEWAY_URL ?? 'http://localhost:8000'

export default defineConfig({
  plugins: [vue(), vuetify({ autoImport: true, styles: { configFile: 'src/styles/settings.scss' } })],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    port: 5173,
    strictPort: true,
    proxy: { '/api': { target: gateway, changeOrigin: false, rewrite: (path) => path.replace(/^\/api/, '') } },
  },
  test: {
    environment: 'jsdom',
    globals: false,
    server: { deps: { inline: ['vuetify'] } },
    include: ['src/**/*.test.ts'],
  },
})
