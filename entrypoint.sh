#!/bin/ash
./imageproxy -addr "0.0.0.0:$PORT" -urlPrefix "$URL_PREFIX" -baseURL "$BASE_URL"