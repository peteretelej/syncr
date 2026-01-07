// pm2 Ecosystem Configuration
// Run with: pm2 start ecosystem.config.js

module.exports = {
  apps: [
    {
      name: 'syncr',
      script: 'src/cli.ts',
      interpreter: 'bun',
      
      // Run sync every hour using cron
      cron_restart: '0 * * * *',
      
      // Don't restart automatically (we use cron instead)
      autorestart: false,
      
      // Watch for config changes (optional)
      watch: false,
      
      // Log configuration
      log_date_format: 'YYYY-MM-DD HH:mm:ss',
      error_file: 'logs/pm2-error.log',
      out_file: 'logs/pm2-out.log',
      merge_logs: true,
      
      // Environment
      env: {
        NODE_ENV: 'production',
      },
      
      // Resource limits
      max_memory_restart: '200M',
      
      // Startup
      wait_ready: false,
      kill_timeout: 5000,
    },
  ],
};
