#!/bin/zsh

export BROWSER_SYNC_HOST=${BROWSER_SYNC_HOST:-drynn.test}
export BROWSER_SYNC_PROXY=http://drynn.test:8989

npx --yes browser-sync start --config browser-sync.config.js
