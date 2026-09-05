module.exports = {
  apps: [
    {
      name: "9router-gateway",
      script: "npx",
      args: "9router start",
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: "300M",
      env: {
        PORT: "20128",
      },
    },
    {
      name: "nexo-api",
      script: "./bin/server",
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: "500M",
      env: {
        APP_ENV: "production",
        APP_PORT: "3001",
      },
    },
  ],
};
