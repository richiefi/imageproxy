package imageproxy

import (
	"net/http"
	"net/url"
	"testing"
)

var emptyOptions = Options{}

func TestOptions_String(t *testing.T) {
	tests := []struct {
		Options Options
		String  string
	}{
		{
			emptyOptions,
			"0x0",
		},
		{
			Options{1, 2, true, 90, true, true, 80, "", false, "", 0, 0, 0, 0, false},
			"1x2,fit,r90,fv,fh,q80",
		},
		{
			Options{0.15, 1.3, false, 45, false, false, 95, "c0ffee", false, "png", 0, 0, 0, 0, false},
			"0.15x1.3,r45,q95,sc0ffee,png",
		},
		{
			Options{0.15, 1.3, false, 45, false, false, 95, "c0ffee", false, "", 100, 200, 0, 0, false},
			"0.15x1.3,r45,q95,sc0ffee,cx100,cy200",
		},
		{
			Options{0.15, 1.3, false, 45, false, false, 95, "c0ffee", false, "png", 100, 200, 300, 400, false},
			"0.15x1.3,r45,q95,sc0ffee,png,cx100,cy200,cw300,ch400",
		},
	}

	for i, tt := range tests {
		if got, want := tt.Options.String(), tt.String; got != want {
			t.Errorf("%d. Options.String returned %v, want %v", i, got, want)
		}
	}
}

func TestParseFormValues(t *testing.T) {
	tests := []struct {
		InputQS string
		Options Options
	}{
		{"", emptyOptions},
		{"x", emptyOptions},
		{"r", emptyOptions},
		{"0", emptyOptions},
		{"crop=,,,,", emptyOptions},

		// size variations
		{"width=1", Options{Width: 1}},
		{"height=1", Options{Height: 1}},
		{"width=1&height=2", Options{Width: 1, Height: 2, Fit: true}},
		{"width=-1&height=-2", Options{Width: -1, Height: -2}},
		{"width=0.1&height=0.2", Options{Width: 0.1, Height: 0.2, Fit: true}},
		{"size=1", Options{Width: 1, Height: 1, Fit: true}},
		{"size=0.1", Options{Width: 0.1, Height: 0.1, Fit: true}},

		// sizes with dpr
		{"width=1&dpr=3", Options{Width: 3}},
		{"height=1&dpr=3", Options{Height: 3}},
		{"width=1&height=2&dpr=3", Options{Width: 3, Height: 6, Fit: true}},
		{"width=-1&height=-2&dpr=3", Options{Width: -3, Height: -6}},
		{"width=0.1&height=0.2&dpr=3", Options{Width: 0.3, Height: 0.6, Fit: true}},
		{"size=1&dpr=3", Options{Width: 3, Height: 3, Fit: true}},
		{"size=0.1&dpr=3", Options{Width: 0.3, Height: 0.3, Fit: true}},

		// additional flags
		{"mode=fit", Options{Fit: true}},
		{"rotate=90", Options{Rotate: 90}},
		{"flip=v", Options{FlipVertical: true}},
		{"flip=h", Options{FlipHorizontal: true}},
		{"format=jpeg", Options{Format: "jpeg"}},

		// mix of valid and invalid flags
		{"FOO=BAR&size=1&BAR=foo&rotate=90&BAZ=DAS", Options{Width: 1, Height: 1, Rotate: 90, Fit: true}},

		// flags, in different orders
		{"quality=70&width=1&height=2&mode=fit&rotate=90&flip=v&flip=h&signature=c0ffee&format=png", Options{1, 2, true, 90, true, true, 70, "c0ffee", false, "png", 0, 0, 0, 0, false}},
		{"rotate=90&flip=h&signature=c0ffee&format=png&quality=90&width=1&height=2&flip=v&mode=fit", Options{1, 2, true, 90, true, true, 90, "c0ffee", false, "png", 0, 0, 0, 0, false}},

		// all flags, in different orders with crop
		{"quality=70&width=1&height=2&mode=fit&crop=100,200,300,400&rotate=90&flip=v&flip=h&signature=c0ffee&format=png", Options{1, 2, true, 90, true, true, 70, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"rotate=90&flip=h&signature=c0ffee&format=png&crop=100,200,300,400&quality=90&width=1&height=2&flip=v&mode=fit", Options{1, 2, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},

		// all flags, in different orders with crop & different resizes
		{"quality=70&crop=100,200,300,400&height=2&mode=fit&rotate=90&flip=v&flip=h&signature=c0ffee&format=png", Options{0, 2, true, 90, true, true, 70, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"crop=100,200,300,400&rotate=90&flip=h&quality=90&signature=c0ffee&format=png&width=1&flip=v&mode=fit", Options{1, 0, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"crop=100,200,300,400&rotate=90&flip=h&signature=c0ffee&flip=v&format=png&quality=90&mode=fit", Options{0, 0, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"crop=100,200,0,400&rotate=90&quality=90&flip=h&signature=c0ffee&format=png&flip=v&mode=fit&width=123&height=321", Options{123, 321, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 0, 400, false}},
		{"flip=v&width=123&height=321&crop=100,200,300,400&quality=90&rotate=90&flip=h&signature=c0ffee&format=png&mode=fit", Options{123, 321, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},
	}

	for _, tt := range tests {
		input, err := url.ParseQuery(tt.InputQS)
		if err != nil {
			panic(err)
		}

		if got, want := ParseFormValues(input, Options{}), tt.Options; !got.Equal(want) {
			t.Errorf("ParseFormValues(%q) returned %#v, want %#v", tt.InputQS, got, want)
		}
	}
}

// Test that request URLs are properly parsed into Options and RemoteURL.  This
// test verifies that invalid remote URLs throw errors, and that valid
// combinations of Options and URL are accept.  This does not exhaustively test
// the various Options that can be specified; see TestParseOptions for that.
func TestNewRequest(t *testing.T) {
	tests := []struct {
		URL         string  // input URL to parse as an imageproxy request
		RemoteURL   string  // expected URL of remote image parsed from input
		Options     Options // expected options parsed from input
		ExpectError bool    // whether an error is expected from NewRequest
	}{
		// invalid URLs
		{"http://localhost/", "", emptyOptions, true},
		{"http://localhost/?size=1", "", emptyOptions, true},
		{"http://localhost/example.com/foo", "", emptyOptions, true},
		{"http://localhost/ftp://example.com/foo", "", emptyOptions, true},

		// invalid options.  These won't return errors, but will not fully parse the options
		{
			"http://localhost/http://example.com/?s",
			"http://example.com/?s", emptyOptions, false,
		},
		{
			"http://localhost/http://example.com/?width=1&height=s",
			"http://example.com/?width=1&height=s", Options{Width: 1}, false,
		},

		// valid URLs. the recognized query parameters are dropped just before querying upstream, so they
		// are present in RemoteURLs in this phase :(
		{
			"http://localhost/http://example.com/foo?baz=baz",
			"http://example.com/foo?baz=baz", emptyOptions, false,
		},
		{
			"http://localhost/http://example.com/foo",
			"http://example.com/foo", emptyOptions, false,
		},
		{
			"http://localhost/http://example.com/foo?width=1&height=2",
			"http://example.com/foo?width=1&height=2", Options{Width: 1, Height: 2, Fit: true}, false,
		},
		{
			"http://localhost/http://example.com/foo?width=1&height=2&bar=baz",
			"http://example.com/foo?width=1&height=2&bar=baz", Options{Width: 1, Height: 2, Fit: true}, false,
		},
		{
			"http://localhost/http:/example.com/foo",
			"http://example.com/foo", emptyOptions, false,
		},
		{
			"http://localhost/http:///example.com/foo",
			"http://example.com/foo", emptyOptions, false,
		},
		{ // escaped path
			"http://localhost/http://example.com/%2C",
			"http://example.com/%2C", emptyOptions, false,
		},

		// valid URLs with the prefix
		{
			"http://localhost/prefix/http://example.com/foo?bar=baz",
			"http://example.com/foo?bar=baz", emptyOptions, false,
		},
		{
			"http://localhost/prefix/http://example.com/foo",
			"http://example.com/foo", emptyOptions, false,
		},
		{
			"http://localhost/prefix/http://example.com/foo?width=1&height=2",
			"http://example.com/foo?width=1&height=2", Options{Width: 1, Height: 2, Fit: true}, false,
		},
		{
			"http://localhost/prefix/http://example.com/foo?width=1&height=2&bar=baz",
			"http://example.com/foo?width=1&height=2&bar=baz", Options{Width: 1, Height: 2, Fit: true}, false,
		},
		{
			"http://localhost/prefix/http:/example.com/foo",
			"http://example.com/foo", emptyOptions, false,
		},
		{
			"http://localhost/prefix/http:///example.com/foo",
			"http://example.com/foo", emptyOptions, false,
		},
		{ // escaped path
			"http://localhost/prefix/http://example.com/%2C",
			"http://example.com/%2C", emptyOptions, false,
		},
	}

	// Try with both versions of the same prefix. The results should be same.
	prefixes := []string{"/prefix/", "/prefix"}

	for _, prefix := range prefixes {
		for _, tt := range tests {
			req, err := http.NewRequest("GET", tt.URL, nil)
			if err != nil {
				t.Errorf("http.NewRequest(%q) returned error: %v", tt.URL, err)
				continue
			}

			// Define that our prefix has to be stripped but does not specify a base URL to be used
			cfg := SourceConfiguration{BaseURL: nil}
			configMap := map[string]*SourceConfiguration{prefix: &cfg}

			r, err := NewRequest(req, configMap)
			if tt.ExpectError {
				if err == nil {
					t.Errorf("NewRequest(%v) did not return expected error", req)
				}
				continue
			} else if err != nil {
				t.Errorf("NewRequest(%v) return unexpected error: %v", req, err)
				continue
			}

			if got, want := r.URL.String(), tt.RemoteURL; got != want {
				t.Errorf("NewRequest(%q) request URL = %v, want %v", tt.URL, got, want)
			}
			if got, want := r.Options, tt.Options; got != want {
				t.Errorf("NewRequest(%q) request options = %v, want %v", tt.URL, got, want)
			}
		}
	}
}

func Test_NewRequest_PrefixAndBaseURL(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost/prefix/baz.jpg?size=123", nil)
	if err != nil {
		t.Errorf("http.NewRequest returned error: %s", err.Error())
		return
	}

	baseURL, err := url.Parse("https://imagehost.invalid/foobar/")
	if err != nil {
		t.Errorf("url.Parse returned error: %s", err.Error())
		return
	}
	cfg := SourceConfiguration{BaseURL: baseURL}

	// Define that our prefix has to be stripped and it specifies a base URL
	configMap := map[string]*SourceConfiguration{
		"/prefix": &cfg,
	}

	r, err := NewRequest(req, configMap)
	if err != nil {
		t.Errorf("NewRequest(%v) return unexpected error: %v", req, err)
		return
	}

	expectedRemoteURL := "https://imagehost.invalid/foobar/baz.jpg?size=123"
	actualRemoteURL := r.URL.String()

	expectedOptions := Options{Width: 123, Height: 123, Fit: true}
	actualOptions := r.Options

	if expectedRemoteURL != actualRemoteURL {
		t.Errorf("NewRequest request URL = %v, want %v", actualRemoteURL, expectedRemoteURL)
	}
	if expectedOptions != actualOptions {
		t.Errorf("NewRequest request options = %v, want %v", actualOptions, expectedOptions)
	}

}

func Test_StripOurOptions_NoOptions(t *testing.T) {
	input := ""
	expected := ""
	actual, err := StripOurOptions(input)
	if err != nil {
		t.Fatalf("caught unexpected error: %s", err.Error())
	}
	if expected != actual {
		t.Fatalf("Got '%s', expecting '%s'", actual, expected)
	}
}

func Test_StripOurOptions_OnlyOurOptions(t *testing.T) {
	input := "mode=jpeg&width=50"
	expected := ""
	actual, err := StripOurOptions(input)
	if err != nil {
		t.Fatalf("caught unexpected error: %s", err.Error())
	}
	if expected != actual {
		t.Fatalf("Got '%s', expecting '%s'", actual, expected)
	}
}

func Test_StripOurOptions_OnlyRemoteoOptions(t *testing.T) {
	input := "id=123&secret_token=aaa"
	actual, err := StripOurOptions(input)
	if err != nil {
		t.Fatalf("caught unexpected error: %s", err.Error())
	}

	// Allow both orders
	for _, allowed := range []string{"id=123&secret_token=aaa", "secret_token=aaa&id=123"} {
		if actual == allowed {
			return
		}
	}

	t.Fatalf("Got '%s', expecting '%s' or equivalent", actual, input)
}

func Test_StripOurOptions_AllKindsOfOptions(t *testing.T) {
	input := "mode=jpeg&id=123"
	expected := "id=123"
	actual, err := StripOurOptions(input)
	if err != nil {
		t.Fatalf("caught unexpected error: %s", err.Error())
	}
	if expected != actual {
		t.Fatalf("Got '%s', expecting '%s'", actual, expected)
	}
}
