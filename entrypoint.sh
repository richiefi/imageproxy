#!/bin/ash
exec ./imageproxy -addr "0.0.0.0:$PORT" -baseURLConfURL "$BASE_URL_CONF_URL" -cache "$CACHE_1" -cache "$CACHE_2" -whitelist "$WHITELIST" -timeout 25s -scaleUp
