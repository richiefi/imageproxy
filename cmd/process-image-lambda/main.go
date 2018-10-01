package main

import (
	"context"

	awslambda "github.com/aws/aws-lambda-go/lambda"
	"go.uber.org/zap"

	"github.com/richiefi/imageproxy"
	"github.com/richiefi/imageproxy/lambda"
)

func HandleRequest(ctx context.Context, req imageproxy.LambdaTransformRequest) (*imageproxy.LambdaTransformResponse, error) {
	resp := imageproxy.LambdaTransformResponse{}

	plainLogger, err := zap.NewProduction()
	if err != nil {
		return &resp, err
	}
	logger := plainLogger.Sugar()
	lex, err := lambda.NewLambdaExecutor(logger)
	if err != nil {
		return &resp, err
	}

	suggestedStatus, upstreamHeader, img, err := lex.DoTransformWithURL(req.URLString, req.Options)

	resp.Status = suggestedStatus
	resp.UpstreamHeader = upstreamHeader
	if err != nil {
		return &resp, nil
	}
	resp.Image = img
	return &resp, nil
}

func main() {
	awslambda.Start(HandleRequest)
}
