
import path from "path"
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
    
  },
  server: {
    // Forward same-origin /api and /swagger to the API so `npm run dev`
    // works against a locally-running backend (http://localhost:2021).
    proxy: {
      "/api": { target: "http://localhost:2021", changeOrigin: true },
      "/swagger": { target: "http://localhost:2021", changeOrigin: true },
    },
  },
})
