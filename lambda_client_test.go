package imageproxy_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/richiefi/imageproxy"
)

func Test_LambdaTransformResponse_UnmarshalB64(t *testing.T) {
	expected := imageproxy.LambdaTransformResponse{
		Status: 200,
		Image:  []byte{0xfe, 0xfa, 0x5c, 0x10, 0x11},
	}
	var actual imageproxy.LambdaTransformResponse
	var input bytes.Buffer

	input.WriteString(`{"st": 200, "eh": {"A": "B"}, "im": "/vpcEBE="}`)

	err := json.NewDecoder(&input).Decode(&actual)
	if err != nil {
		t.Fatal("error caught", err)
	}
	if expected.Status != actual.Status {
		t.Fatal("unexpected status", expected.Status)
	}

	if len(expected.Image) != len(actual.Image) {
		t.Fatal("different length on images", len(expected.Image), len(actual.Image))
	}
	for i := range expected.Image {
		if expected.Image[i] != actual.Image[i] {
			t.Fatal("images not equal", i)
		}
	}
}

func Test_LambdaTransformResponse_UnmarshalA85(t *testing.T) {
	expected := imageproxy.LambdaTransformResponse{
		Status: 200,
		Image:  []byte{0xfe, 0xfa, 0x5c, 0x10, 0x11},
	}
	var actual imageproxy.LambdaTransformResponse
	var input bytes.Buffer

	input.WriteString(`{"st": 200, "eh": {"A": "B"}, "im": "rq]k2&H"}`)

	err := json.NewDecoder(&input).Decode(&actual)
	if err != nil {
		t.Fatal("error caught", err)
	}
	if expected.Status != actual.Status {
		t.Fatal("unexpected status", expected.Status)
	}
	if len(expected.Image) != len(actual.Image) {
		t.Fatal("different length on images", len(expected.Image), len(actual.Image))
	}
	for i := range expected.Image {
		if expected.Image[i] != actual.Image[i] {
			t.Fatal("images not equal", i)
		}
	}
}

func Test_LambdaTransformResponse_Marshal(t *testing.T) {
	resp := imageproxy.LambdaTransformResponse{
		Status:         200,
		UpstreamHeader: http.Header{},
		Image:          []byte{0xfe, 0xfa, 0x5c, 0x10, 0x11},
	}
	expected := `{"st":200,"uh":{},"im":"rq]k2&H"}
`

	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	enc.SetEscapeHTML(false)
	err := enc.Encode(resp)
	if err != nil {
		t.Fatal("error caught", err)
	}

	actual := buffer.String()
	if actual != expected {
		t.Fatal("wrong encode", actual)
	}
}
