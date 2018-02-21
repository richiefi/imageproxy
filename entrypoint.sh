#!/bin/ash
export LD_LIBRARY_PATH="/usr/local/lib64/:$LD_LIBRARY_PATH"
exec ./imageproxy -addr "0.0.0.0:$PORT" -urlPrefix "$URL_PREFIX" -baseURL "$BASE_URL" -cache "$CACHE" -whitelist "$WHITELIST"
