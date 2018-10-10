package imageproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/richiefi/imageproxy/options"
)

// URLError reports a malformed URL error.
type URLError struct {
	Message string
	URL     *url.URL
}

func (e URLError) Error() string {
	return fmt.Sprintf("malformed URL %q: %s", e.URL, e.Message)
}

type SourceConfiguration struct {
	BaseURL          *url.URL
	DefaultOptions   options.Options
	StripPublication bool
}

func (conf *SourceConfiguration) UnmarshalJSON(bytes []byte) error {
	/*
		Make it possible to unmarshal bytes into a struct with a *url.URL field by first unmarshaling into
		a struct without *url.URLs and then parsing the URL.
	*/
	var confWithString struct {
		BaseURL          string          `json:"base_url"`
		DefaultOptions   options.Options `json:"default_options"`
		StripPublication bool            `json:"strip_publication"`
	}
	err := json.Unmarshal(bytes, &confWithString)
	if err != nil {
		return err
	}
	baseURL, err := url.Parse(confWithString.BaseURL)
	if err != nil {
		return err
	}

	conf.BaseURL = baseURL
	conf.DefaultOptions = confWithString.DefaultOptions
	conf.StripPublication = confWithString.StripPublication
	return nil
}

func StripOurOptions(rawQuery string) (string, error) {
	// Delete our options. This is useful when the request is pushed upstream.
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", err
	}
	newValues := make(url.Values, len(values))
	for key := range values {
		switch key {
		// Do not copy our values
		case "mode":
		case "flip":
		case "format":
		case "rotate":
		case "quality":
		case "signature":
		case "crop":
		case "width":
		case "height":
		case "size":

		// Do copy other values
		default:
			newValues[key] = values[key]
		}
	}
	return newValues.Encode(), nil
}

// Request is an imageproxy request which includes a remote URL of an image to
// proxy, and an optional set of transformations to perform.
type Request struct {
	URL      *url.URL        // URL of the image to proxy
	Options  options.Options // Image transformation to perform
	Original *http.Request   // The original HTTP request
}

// String returns the request URL as a string, with r.Options encoded in the
// URL fragment.
func (r Request) String() string {
	u := *r.URL
	u.Fragment = r.Options.String()
	return u.String()
}

// NewRequest parses an http.Request into an imageproxy Request.  Options and
// the remote image URL are specified in the request path, formatted as:
// /{options}/{remote_url}.  Options may be omitted, so a request path may
// simply contain /{remote_url}.  The remote URL must be an absolute "http" or
// "https" URL, should not be URL encoded, and may contain a query string.
//
// Assuming an imageproxy server running on localhost, the following are all
// valid imageproxy requests:
//
// 	http://localhost/100x200/http://example.com/image.jpg
// 	http://localhost/100x200,r90/http://example.com/image.jpg?foo=bar
// 	http://localhost//http://example.com/image.jpg
// 	http://localhost/http://example.com/image.jpg
func NewRequest(r *http.Request, prefixesToConfigs map[string]*SourceConfiguration) (*Request, error) {
	var err error
	req := &Request{Original: r}

	req.URL, err = buildFinalAbsoluteURL(prefixesToConfigs, r.URL)
	if err != nil {
		return nil, err
	}

	if !req.URL.IsAbs() {
		return nil, URLError{"must provide absolute remote URL", r.URL}
	}

	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, URLError{"remote URL must have http or https scheme", r.URL}
	}

	_, config := bestMatchingConfig(prefixesToConfigs, r.URL)
	var defaultOptions options.Options
	if config != nil {
		defaultOptions = config.DefaultOptions
	} else {
		defaultOptions = options.Options{}
	}

	// Options are now based on query strings params of the original request
	err = r.ParseForm()
	if err != nil {
		return nil, err
	}
	req.Options = options.ParseFormValues(r.Form, defaultOptions)

	req.URL.RawQuery = r.URL.RawQuery

	return req, nil
}

func buildFinalAbsoluteURL(prefixesToConfigs map[string]*SourceConfiguration, originalURL *url.URL) (*url.URL, error) {
	path := originalURL.EscapedPath()[1:]

	matchingPrefix, config := bestMatchingConfig(prefixesToConfigs, originalURL)

	if config != nil {
		urlPrefixWithoutTail := strings.TrimRight(matchingPrefix, "/")
		strippedPath := path[len(urlPrefixWithoutTail):] // strip the prefix

		if len(strippedPath) < 1 {
			return nil, fmt.Errorf("nothing left of path (%s) after removing prefix", path)
		}

		// Publication can be signaled as the first slash-limited part after base URL. Strip it if asked so in conf.
		if config.StripPublication {
			// Drop leading slash
			strippedPath = strings.TrimLeft(strippedPath, "/")

			// Cut at first remaining slash
			parts := strings.SplitAfterN(strippedPath, "/", 2)
			if len(parts) < 1 {
				return nil, fmt.Errorf("nothing left of path (%s) after removing prefix and publication", path)
			}
			strippedPath = parts[1]
		}

		finalURL, err := parseURL(strippedPath)

		// Add parsed URL to the matching base URL if there is one
		if config.BaseURL != nil {
			finalURL = config.BaseURL.ResolveReference(finalURL)
		}
		return finalURL, err

	} else {
		// Not a single matching prefix was found.
		return parseURL(path)
	}
}

func bestMatchingConfig(prefixesToConfigs map[string]*SourceConfiguration, originalURL *url.URL) (string, *SourceConfiguration) {
	bestMatchLen := -1
	bestMatchPrefix := ""
	var bestConfig *SourceConfiguration = nil

	for urlPrefix, config := range prefixesToConfigs {
		urlPrefixWithoutTail := strings.TrimRight(urlPrefix, "/")

		if strings.HasPrefix(originalURL.EscapedPath(), urlPrefixWithoutTail) {
			matchLen := len(urlPrefixWithoutTail)
			if matchLen < bestMatchLen {
				continue
			}

			// A better thing found
			bestMatchLen = matchLen
			bestMatchPrefix = urlPrefix
			bestConfig = config
		}
	}
	return bestMatchPrefix, bestConfig
}

var reCleanedURL = regexp.MustCompile(`^(https?):/+([^/])`)

// parseURL parses s as a URL, handling URLs that have been munged by
// path.Clean or a webserver that collapses multiple slashes.
func parseURL(s string) (*url.URL, error) {
	s = reCleanedURL.ReplaceAllString(s, "$1://$2")
	return url.Parse(s)
}
