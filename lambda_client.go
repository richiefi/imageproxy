package imageproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/richiefi/imageproxy/options"
)

type LambdaTransformRequest struct {
	URLString string          `json:"u"`
	Options   options.Options `json:"o"`
}

type LambdaTransformResponse struct {
	Status         int         `json:"st"`
	UpstreamHeader http.Header `json:"uh"`
	Image          []byte      `json:"im"`
}

type LambdaClient interface {
	TransformWithURL(*url.URL, options.Options) (int, http.Header, []byte, error)
}

type lambdaClient struct {
	c            *lambda.Lambda
	functionName string
}

func NewLambdaClient(functionName string) (LambdaClient, error) {
	session, err := awssession.NewSession()
	if err != nil {
		return nil, err
	}
	client := lambdaClient{}
	client.c = lambda.New(session)
	client.functionName = functionName
	return &client, nil
}

func (c *lambdaClient) TransformWithURL(u *url.URL, options options.Options) (int, http.Header, []byte, error) {
	// TransformWithURL will call Lambda synchronously and executes DoTransformWithURL there
	if u == nil {
		return 0, nil, nil, fmt.Errorf("URL can't be nil")
	}

	reqPayload, err := json.Marshal(LambdaTransformRequest{
		URLString: u.String(),
		Options:   options,
	})
	if err != nil {
		return 0, nil, nil, err
	}

	invokeOutput, err := c.c.Invoke(&lambda.InvokeInput{
		FunctionName:   aws.String(c.functionName),
		InvocationType: aws.String("RequestResponse"), // this makes it synchronous
		Payload:        reqPayload,
	})
	if err != nil {
		return 0, nil, nil, err
	}

	var resp LambdaTransformResponse
	err = json.Unmarshal(invokeOutput.Payload, &resp)
	if err != nil {
		return 0, nil, nil, err
	}
	return resp.Status, resp.UpstreamHeader, resp.Image, nil
}
