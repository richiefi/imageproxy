package imageproxy

import (
	"bufio"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/richiefi/imageproxy/options"
)

func logger() *zap.SugaredLogger {
	plainLogger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	return plainLogger.Sugar()
}

func TestCopyHeader(t *testing.T) {
	tests := []struct {
		dst, src http.Header
		keys     []string
		want     http.Header
	}{
		// empty
		{http.Header{}, http.Header{}, nil, http.Header{}},
		{http.Header{}, http.Header{}, []string{}, http.Header{}},
		{http.Header{}, http.Header{}, []string{"A"}, http.Header{}},

		// nothing to copy
		{
			dst:  http.Header{"A": []string{"a1"}},
			src:  http.Header{},
			keys: nil,
			want: http.Header{"A": []string{"a1"}},
		},
		{
			dst:  http.Header{},
			src:  http.Header{"A": []string{"a"}},
			keys: []string{"B"},
			want: http.Header{},
		},

		// copy headers
		{
			dst:  http.Header{},
			src:  http.Header{"A": []string{"a"}},
			keys: nil,
			want: http.Header{"A": []string{"a"}},
		},
		{
			dst:  http.Header{"A": []string{"a"}},
			src:  http.Header{"B": []string{"b"}},
			keys: nil,
			want: http.Header{"A": []string{"a"}, "B": []string{"b"}},
		},
		{
			dst:  http.Header{"A": []string{"a"}},
			src:  http.Header{"B": []string{"b"}, "C": []string{"c"}},
			keys: []string{"B"},
			want: http.Header{"A": []string{"a"}, "B": []string{"b"}},
		},
		{
			dst:  http.Header{"A": []string{"a1"}},
			src:  http.Header{"A": []string{"a2"}},
			keys: nil,
			want: http.Header{"A": []string{"a1", "a2"}},
		},
	}

	for _, tt := range tests {
		// copy dst map
		got := make(http.Header)
		for k, v := range tt.dst {
			got[k] = v
		}

		copyHeader(got, tt.src, tt.keys...)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("copyHeader(%v, %v, %v) returned %v, want %v", tt.dst, tt.src, tt.keys, got, tt.want)
		}

	}
}

func TestAllowed(t *testing.T) {
	whitelist := []string{"good"}
	key := []byte("c0ffee")

	genRequest := func(headers map[string]string) *http.Request {
		req := &http.Request{Header: make(http.Header)}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		return req
	}

	tests := []struct {
		url       string
		options   options.Options
		whitelist []string
		referrers []string
		key       []byte
		request   *http.Request
		allowed   bool
	}{
		// no whitelist or signature key
		{"http://test/image", emptyOptions, nil, nil, nil, nil, true},

		// whitelist
		{"http://good/image", emptyOptions, whitelist, nil, nil, nil, true},
		{"http://bad/image", emptyOptions, whitelist, nil, nil, nil, false},

		// referrer
		{"http://test/image", emptyOptions, nil, whitelist, nil, genRequest(map[string]string{"Referer": "http://good/foo"}), true},
		{"http://test/image", emptyOptions, nil, whitelist, nil, genRequest(map[string]string{"Referer": "http://bad/foo"}), false},
		{"http://test/image", emptyOptions, nil, whitelist, nil, genRequest(map[string]string{"Referer": "MALFORMED!!"}), false},
		{"http://test/image", emptyOptions, nil, whitelist, nil, genRequest(map[string]string{}), false},

		// signature key
		{"http://test/image", options.Options{Signature: "NDx5zZHx7QfE8E-ijowRreq6CJJBZjwiRfOVk_mkfQQ="}, nil, nil, key, nil, true},
		{"http://test/image", options.Options{Signature: "deadbeef"}, nil, nil, key, nil, false},
		{"http://test/image", emptyOptions, nil, nil, key, nil, false},

		// whitelist and signature
		{"http://good/image", emptyOptions, whitelist, nil, key, nil, true},
		{"http://bad/image", options.Options{Signature: "gWivrPhXBbsYEwpmWAKjbJEiAEgZwbXbltg95O2tgNI="}, nil, nil, key, nil, true},
		{"http://bad/image", emptyOptions, whitelist, nil, key, nil, false},
	}

	for _, tt := range tests {
		p := NewProxy(nil, "func", logger())
		p.Whitelist = tt.whitelist
		p.SignatureKey = tt.key
		p.Referrers = tt.referrers

		u, err := url.Parse(tt.url)
		if err != nil {
			t.Errorf("error parsing url %q: %v", tt.url, err)
		}
		req := &Request{u, tt.options, tt.request}
		if got, want := p.allowed(req), tt.allowed; (got == nil) != want {
			t.Errorf("allowed(%q) returned %v, want %v.\nTest struct: %#v", req, got, want, tt)
		}
	}
}

func TestValidHost(t *testing.T) {
	whitelist := []string{"a.test", "*.b.test", "*c.test"}

	tests := []struct {
		url   string
		valid bool
	}{
		{"http://a.test/image", true},
		{"http://x.a.test/image", false},

		{"http://b.test/image", true},
		{"http://x.b.test/image", true},
		{"http://x.y.b.test/image", true},

		{"http://c.test/image", false},
		{"http://xc.test/image", false},
		{"/image", false},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.url)
		if err != nil {
			t.Errorf("error parsing url %q: %v", tt.url, err)
		}
		if got, want := validHost(whitelist, u), tt.valid; got != want {
			t.Errorf("validHost(%v, %q) returned %v, want %v", whitelist, u, got, want)
		}
	}
}

func TestValidSignature(t *testing.T) {
	key := []byte("c0ffee")

	tests := []struct {
		url     string
		options options.Options
		valid   bool
	}{
		{"http://test/image", options.Options{Signature: "NDx5zZHx7QfE8E-ijowRreq6CJJBZjwiRfOVk_mkfQQ="}, true},
		{"http://test/image", options.Options{Signature: "NDx5zZHx7QfE8E-ijowRreq6CJJBZjwiRfOVk_mkfQQ"}, true},
		{"http://test/image", emptyOptions, false},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.url)
		if err != nil {
			t.Errorf("error parsing url %q: %v", tt.url, err)
		}
		req := &Request{u, tt.options, &http.Request{}}
		if got, want := validSignature(key, req), tt.valid; got != want {
			t.Errorf("validSignature(%v, %q) returned %v, want %v", key, u, got, want)
		}
	}
}

func TestShould304(t *testing.T) {
	tests := []struct {
		req      string
		respEtag string
		is304    bool
	}{
		{ // etag match
			"GET / HTTP/1.1\nIf-None-Match: \"v\"\n\n",
			`"v"`,
			true,
		},

		// mismatches
		{
			"GET / HTTP/1.1\n\n",
			"",
			false,
		},
		{
			"GET / HTTP/1.1\n\n",
			`"v"`,
			false,
		},
		{
			"GET / HTTP/1.1\nIf-None-Match: \"v\"\n\n",
			"",
			false,
		},
		{
			"GET / HTTP/1.1\nIf-None-Match: \"a\"\n\n",
			`"b"`,
			false,
		},
	}

	for _, tt := range tests {
		buf := bufio.NewReader(strings.NewReader(tt.req))
		req, err := http.ReadRequest(buf)
		if err != nil {
			t.Errorf("http.ReadRequest(%q) returned error: %v", tt.req, err)
		}

		if got, want := should304(req, tt.respEtag), tt.is304; got != want {
			t.Errorf("should304(%q, %q) returned: %v, want %v", tt.req, tt.respEtag, got, want)
		}
	}
}
