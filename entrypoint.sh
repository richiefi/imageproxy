#!/bin/ash
exec ./imageproxy -addr "0.0.0.0:$PORT" -urlPrefix "$URL_PREFIX" -baseURL "$BASE_URL" -forbidAbsoluteURLs "$FORBID_ABSOLUTE_URLS" -cache "$CACHE"
