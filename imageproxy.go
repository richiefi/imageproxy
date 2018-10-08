// Package imageproxy provides an image proxy server.  For typical use of
// creating and using a Proxy, see cmd/imageproxy/main.go.
package imageproxy

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	"go.uber.org/zap"

	"github.com/richiefi/imageproxy/options"
	tphttp "github.com/richiefi/imageproxy/third_party/http"
)

const cacheTags = "imageproxy,imageproxy-1"

// Overridden in prod by linker
var buildVersion = "dev"

// Proxy serves image requests.
type Proxy struct {
	logger *zap.SugaredLogger

	Client *http.Client // client used to fetch remote URLs

	// Whitelist specifies a list of remote hosts that images can be
	// proxied from.  An empty list means all hosts are allowed.
	Whitelist []string

	// Referrers, when given, requires that requests to the image
	// proxy come from a referring host. An empty list means all
	// hosts are allowed.
	Referrers []string

	PrefixesToConfigs map[string]*SourceConfiguration

	// SignatureKey is the HMAC key used to verify signed requests.
	SignatureKey []byte

	// Allow images to scale beyond their original dimensions.
	ScaleUp bool

	// Timeout specifies a time limit for requests served by this Proxy.
	// If a call runs for longer than its time limit, a 504 Gateway Timeout
	// response is returned.  A Timeout of zero means no timeout.
	Timeout time.Duration
}

// NewProxy constructs a new proxy.  The provided http RoundTripper will be
// used to fetch remote URLs.  If nil is provided, http.DefaultTransport will
// be used.
func NewProxy(transport http.RoundTripper, lambdaFunctionName string, logger *zap.SugaredLogger) *Proxy {
	logger.Infow("Initializing imageproxy",
		"buildVersion", buildVersion,
	)

	if transport == nil {
		transport = http.DefaultTransport
	}

	proxy := &Proxy{
		logger: logger,
	}

	client := new(http.Client)
	client.Transport = &TransformingTransport{
		logger:             logger,
		headClient:         &http.Client{Timeout: 10 * time.Second},
		lambdaFunctionName: lambdaFunctionName,
	}

	proxy.Client = client
	proxy.Client.Timeout = 10 * time.Second

	return proxy
}

// ServeHTTP handles incoming requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Respond to health checks
	if r.URL.Path == "/" || r.URL.Path == "/health-check" {
		fmt.Fprint(w, "OK")
		return
	}

	// Ignore certain urls without actually parsing them
	if r.URL.Path == "/favicon.ico" || r.URL.Path == "/apple-touch-icon.png" || r.URL.Path == "/apple-touch-icon-precomposed.png" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Not found")
		return
	}

	var h http.Handler = http.HandlerFunc(p.serveImage)
	if p.Timeout > 0 {
		h = tphttp.TimeoutHandler(h, p.Timeout, "Gateway timeout waiting for remote resource.")
	}

	h = WithLogging(h, p.logger)
	h.ServeHTTP(w, r)
}

// serveImage handles incoming requests for proxied images.
func (p *Proxy) serveImage(w http.ResponseWriter, r *http.Request) {
	req, err := NewRequest(r, p.PrefixesToConfigs)
	if err != nil {
		p.logger.Infow("invalid request URL",
			"error", err.Error(),
		)
		msg := fmt.Sprintf("invalid request URL: %s", err.Error())
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// assign static settings from proxy to req.Options
	req.Options.ScaleUp = p.ScaleUp

	if err := p.allowed(req); err != nil {
		p.logger.Infow("Generated request did not pass validation",
			"error", err.Error(),
			"req.URL", req.URL,
		)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	resp, err := p.Client.Get(req.String())
	if err != nil {
		p.logger.Infow("Error fetching a remote image",
			"error", err.Error(),
			"req.String()", req.String(),
		)
		raven.CaptureError(err, nil)
		msg := fmt.Sprintf("error fetching remote image: %s", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	p.logger.Debugw("About to respond",
		"req.String()", req.String(),
	)

	copyHeader(w.Header(), resp.Header, "Cache-Control", "Last-Modified", "Expires", "Link")

	// Set Cache-Tag values to make it possible to detect and purge responses created by this app
	resp.Header.Set("Cache-Tag", cacheTags)

	// Enable CORS for 3rd party applications
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Etag business
	remoteEtag := resp.Header.Get("Etag")
	if remoteEtag != "" {
		responseEtag := semanticEtag(remoteEtag, req.Options)
		etagHeader := fmt.Sprintf(`W/"%s"`, responseEtag)

		w.Header().Set("ETag", etagHeader)

		if should304(r, responseEtag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Move on with other-than-304 response
	copyHeader(w.Header(), resp.Header, "Content-Length", "Content-Type")

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// copyHeader copies header values from src to dst, adding to any existing
// values with the same header name.  If keys is not empty, only those header
// keys will be copied.
func copyHeader(dst, src http.Header, keys ...string) {
	if len(keys) == 0 {
		for k, _ := range src {
			keys = append(keys, k)
		}
	}
	for _, key := range keys {
		k := http.CanonicalHeaderKey(key)
		for _, v := range src[k] {
			dst.Add(k, v)
		}
	}
}

func semanticEtag(remoteEtag string, options options.Options) string {
	h := md5.New()
	fmt.Fprintf(h, "%s%s%s", remoteEtag, options.String(), buildVersion)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// allowed determines whether the specified request contains an allowed
// referrer, host, and signature.  It returns an error if the request is not
// allowed.
func (p *Proxy) allowed(r *Request) error {
	if len(p.Referrers) > 0 && !validReferrer(p.Referrers, r.Original) {
		return fmt.Errorf("request does not contain an allowed referrer: %v", r)
	}

	if len(p.Whitelist) == 0 && len(p.SignatureKey) == 0 {
		return nil // no whitelist or signature key, all requests accepted
	}

	if len(p.Whitelist) > 0 && validHost(p.Whitelist, r.URL) {
		return nil
	}

	if len(p.SignatureKey) > 0 && validSignature(p.SignatureKey, r) {
		return nil
	}

	return fmt.Errorf("request does not contain an allowed host or valid signature: %v", r)
}

// validHost returns whether the host in u matches one of hosts.
func validHost(hosts []string, u *url.URL) bool {
	for _, host := range hosts {
		if u.Host == host {
			return true
		}
		if strings.HasPrefix(host, "*.") && strings.HasSuffix(u.Host, host[2:]) {
			return true
		}
	}

	return false
}

// returns whether the referrer from the request is in the host list.
func validReferrer(hosts []string, r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Referer"))
	if err != nil { // malformed or blank header, just deny
		return false
	}

	return validHost(hosts, u)
}

// validSignature returns whether the request signature is valid.
func validSignature(key []byte, r *Request) bool {
	sig := r.Options.Signature
	if m := len(sig) % 4; m != 0 { // add padding if missing
		sig += strings.Repeat("=", 4-m)
	}

	got, err := base64.URLEncoding.DecodeString(sig)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(r.URL.String()))
	want := mac.Sum(nil)

	return hmac.Equal(got, want)
}

func should304(req *http.Request, responseEtag string) bool {
	if responseEtag == "" {
		return false
	}

	ifNoneMatch := req.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}

	candidates := strings.Split(ifNoneMatch, ",")
	for _, candidate := range candidates {
		stripped := strings.TrimSpace(candidate)
		stripped = strings.TrimPrefix(stripped, "W/")

		if stripped == responseEtag {
			return true
		}
	}
	return false
}

// TransformingTransport is an implementation of http.RoundTripper that
// optionally transforms images using the options specified in the request URL
// fragment.
type TransformingTransport struct {
	logger             *zap.SugaredLogger
	lambdaFunctionName string
	headClient         *http.Client
}

// RoundTrip implements the http.RoundTripper interface.
func (t *TransformingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error

	u := *req.URL
	u.Fragment = ""

	// Drop recognized options at this point, so they are not sent upstream
	u.RawQuery, err = StripOurOptions(u.RawQuery)
	if err != nil {
		t.logger.Warnw("Unable to re-parse url query",
			"u.RawQuery", u.RawQuery,
			"error", err.Error(),
		)
	}

	opt := options.ParseOptions(req.URL.Fragment)

	// Try to drop extension (like .jpg, .png...)
	u.Path = strings.TrimSuffix(u.Path, path.Ext(u.Path))

	var status int
	var upstreamHeader http.Header
	var img []byte

	// Try HTTP HEAD to see if it's completely futile to start the Lambda
	resp, err := t.headClient.Head(u.String())
	if err == nil {
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			t.logger.Warnw("Error status for HTTP HEAD, not invoking Lambda",
				"StatusCode", resp.StatusCode,
			)
			status = resp.StatusCode
			upstreamHeader = resp.Header
		} else {
			// No errors and ok-ish status code. Do transform.
			lambdaClient, err := NewLambdaClient(t.lambdaFunctionName)
			if err != nil {
				t.logger.Warnw("Could not initialize Lambda client",
					"Error", err.Error(),
				)
				return nil, err
			}

			t.logger.Infow("Calling Transform over Lambda",
				"options fragment", req.URL.Fragment,
			)

			status, upstreamHeader, img, err = lambdaClient.TransformWithURL(&u, opt)
			if err != nil {
				t.logger.Warnw("Error transforming image",
					"error", err.Error(),
					"opt", opt,
				)
				img = []byte{}
			}
		}
	} else {
		t.logger.Warnw("HTTP HEAD returned an error, not invoking Lambda",
			"Error", err.Error(),
		)
	}

	if status <= 0 {
		status = 500
	}

	// replay response with transformed image and updated content length
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "HTTP/1.1 %d\n", status)
	upstreamHeader.WriteSubset(buf, map[string]bool{
		"Content-Length": true,
		// exclude Content-Type header if the format may have changed during transformation
		"Content-Type": opt.Format != "" || upstreamHeader.Get("Content-Type") == "image/webp" || upstreamHeader.Get("Content-Type") == "image/tiff",
	})
	fmt.Fprintf(buf, "Content-Length: %d\n\n", len(img))
	buf.Write(img)

	return http.ReadResponse(bufio.NewReader(buf), req)
}
