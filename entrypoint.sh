#!/bin/ash
export LD_LIBRARY_PATH="/usr/local/lib64/:$LD_LIBRARY_PATH"
exec ./imageproxy -addr "0.0.0.0:$PORT" -baseURLConfURL "$BASE_URL_CONF_URL" -cache "$CACHE_1" -cache "$CACHE_2" -whitelist "$WHITELIST" -maxConcurrency "$MAX_CONCURRENCY" -timeout 25s
