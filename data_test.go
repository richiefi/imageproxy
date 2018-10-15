package imageproxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/richiefi/imageproxy/options"
)

var emptyOptions = options.Options{}

// Test that request URLs are properly parsed into Options and RemoteURL.  This
// test verifies that invalid remote URLs throw errors, and that valid
// combinations of Options and URL are accept.  This does not exhaustively test
// the various Options that can be specified; see TestParseOptions for that.
func TestNewRequest(t *testing.T) {
	tests := []struct {
		URL         string          // input URL to parse as an imageproxy request
		RemoteURL   string          // expected URL of remote image parsed from input
		Options     options.Options // expected options parsed from input
		ExpectError bool            // whether an error is expected from NewRequest
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
			"http://example.com/?width=1&height=s", options.Options{Width: 1}, false,
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
			"http://example.com/foo?width=1&height=2", options.Options{Width: 1, Height: 2, Fit: true}, false,
		},
		{
			"http://localhost/http://example.com/foo?width=1&height=2&bar=baz",
			"http://example.com/foo?width=1&height=2&bar=baz", options.Options{Width: 1, Height: 2, Fit: true}, false,
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
			"http://example.com/foo?width=1&height=2", options.Options{Width: 1, Height: 2, Fit: true}, false,
		},
		{
			"http://localhost/prefix/http://example.com/foo?width=1&height=2&bar=baz",
			"http://example.com/foo?width=1&height=2&bar=baz", options.Options{Width: 1, Height: 2, Fit: true}, false,
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

	expectedOptions := options.Options{Width: 123, Height: 123, Fit: true}
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
