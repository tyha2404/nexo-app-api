module.exports = {
  apps: [
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
