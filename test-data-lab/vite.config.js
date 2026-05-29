const proxyTarget = process.env.VITE_PROXY_TARGET || "http://localhost:8080";
const port = Number(process.env.VITE_TEST_DATA_LAB_PORT || 5190);

export default {
  server: {
    host: "127.0.0.1",
    port,
    proxy: {
      "/api": {
        target: proxyTarget,
        changeOrigin: true,
      },
      "/healthz": {
        target: proxyTarget,
        changeOrigin: true,
      },
    },
  },
};
