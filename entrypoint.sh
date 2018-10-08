#!/bin/ash
exec ./imageproxy -addr "0.0.0.0:$PORT" -lambdaFunctionName "$LAMBDA_FUNCTION_NAME" -baseURLConfURL "$BASE_URL_CONF_URL" -whitelist "$WHITELIST" -timeout 25s
