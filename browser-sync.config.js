module.exports = {
  host: process.env.BROWSER_SYNC_HOST || "drynn.test",
  proxy: process.env.BROWSER_SYNC_PROXY || "http://drynn.test:8989",
  port: 3000,
  ui: false,
  notify: false,
  open: false,
  reloadDebounce: 300,
  files: [
    "web/**/*.css",
    "web/**/*.html",
    "tmp/drynn",
  ],
  watchOptions: {
    ignoreInitial: true,
  },
};
