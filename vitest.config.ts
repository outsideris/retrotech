import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    // Default to Node; component tests opt into jsdom via `// @vitest-environment jsdom`.
    environment: 'node',
  },
})
