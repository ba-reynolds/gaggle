
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
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return;
          if (id.includes("recharts")) return "charts";
          if (id.includes("@radix-ui")) return "radix";
          if (id.includes("react-router")) return "router";
          if (id.includes("@tanstack")) return "query";
          if (
            id.includes("/node_modules/react/") ||
            id.includes("/node_modules/react-dom/") ||
            id.includes("/node_modules/scheduler/")
          )
            return "react";
          // Everything else in node_modules → vendor (axios, date-fns, zod, lucide, RHF, etc.)
          return "vendor";
        },
      },
    },
    chunkSizeWarningLimit: 600,
  },
})
