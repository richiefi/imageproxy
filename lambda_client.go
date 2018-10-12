package imageproxy

import (
	"bytes"
	"encoding/ascii85"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

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
	Status         int
	UpstreamHeader http.Header
	Image          []byte
}

func (r LambdaTransformResponse) MarshalJSON() ([]byte, error) {
	var withString struct {
		Status         int         `json:"st"`
		UpstreamHeader http.Header `json:"uh"`
		ImageString    string      `json:"im"` // []byte would trigger implicit base64 ops, our output is ascii85
	}
	withString.Status = r.Status
	withString.UpstreamHeader = r.UpstreamHeader

	var buffer bytes.Buffer
	var err error
	a85Enc := ascii85.NewEncoder(&buffer)
	_, err = a85Enc.Write(r.Image)
	if err != nil {
		return nil, err
	}
	err = a85Enc.Close()
	if err != nil {
		return nil, err
	}
	withString.ImageString = buffer.String()

	var jsonBuffer bytes.Buffer
	jsonEnc := json.NewEncoder(&jsonBuffer)
	jsonEnc.SetEscapeHTML(false) // No need to prepare this to be accidentally parsed as HTNL
	err = jsonEnc.Encode(withString)
	if err != nil {
		return nil, err
	}
	return jsonBuffer.Bytes(), nil
}

func (r *LambdaTransformResponse) UnmarshalJSON(bs []byte) error {
	var withString struct {
		Status         int         `json:"st"`
		UpstreamHeader http.Header `json:"uh"`
		ImageString    string      `json:"im"` // []byte would trigger implicit base64 ops, and our input could be ascii85
	}
	err := json.Unmarshal(bs, &withString)
	if err != nil {
		return err
	}

	// Easy cases
	r.Status = withString.Status
	r.UpstreamHeader = withString.UpstreamHeader

	var b64Err error
	r.Image, b64Err = ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(withString.ImageString)))
	if b64Err != nil {
		// Try ascii85
		var a85Err error
		r.Image, a85Err = ioutil.ReadAll(ascii85.NewDecoder(strings.NewReader(withString.ImageString)))
		if a85Err != nil {
			return fmt.Errorf("multifail: %s; %s", b64Err.Error(), a85Err.Error())
		}
	}
	return nil
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
