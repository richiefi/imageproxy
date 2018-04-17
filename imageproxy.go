// Package imageproxy provides an image proxy server.  For typical use of
// creating and using a Proxy, see cmd/imageproxy/main.go.
package imageproxy

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gregjones/httpcache"
	"go.uber.org/zap"

	tphttp "github.com/richiefi/imageproxy/third_party/http"
)

const cacheTags = "imageproxy,imageproxy-1"

// Proxy serves image requests.
type Proxy struct {
	logger *zap.SugaredLogger

	Client *http.Client // client used to fetch remote URLs
	Cache  Cache        // cache used to cache responses

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
func NewProxy(transport http.RoundTripper, cache Cache, maxConcurrency int, logger *zap.SugaredLogger) *Proxy {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if cache == nil {
		cache = NopCache
	}

	proxy := &Proxy{
		Cache:  cache,
		logger: logger,
	}

	pool := make(chan bool, maxConcurrency)

	client := new(http.Client)
	client.Transport = &httpcache.Transport{
		Transport: &TransformingTransport{
			Transport:     transport,
			CachingClient: client,
			logger:        logger,
			pool:          pool,
		},
		Cache:               cache,
		MarkCachedResponses: true,
	}

	proxy.Client = client

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
		msg := fmt.Sprintf("error fetching remote image: %s", err.Error())
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	cached := resp.Header.Get(httpcache.XFromCache)
	p.logger.Debugw("About to respond",
		"req.String()", req.String(),
		"from cache", cached == "1",
	)

	copyHeader(w.Header(), resp.Header, "Cache-Control", "Last-Modified", "Expires", "Link")

	// Set Cache-Tag values to make it possible to detect and purge responses created by this app
	resp.Header.Set("Cache-Tag", cacheTags)

	if should304(r, resp) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	copyHeader(w.Header(), resp.Header, "Content-Length", "Content-Type")

	//Enable CORS for 3rd party applications
	w.Header().Set("Access-Control-Allow-Origin", "*")

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

// should304 returns whether we should send a 304 Not Modified in response to
// req, based on the response resp.  This is determined using the last modified
// time and the entity tag of resp.
func should304(req *http.Request, resp *http.Response) bool {
	// TODO(willnorris): if-none-match header can be a comma separated list
	// of multiple tags to be matched, or the special value "*" which
	// matches all etags
	etag := resp.Header.Get("Etag")
	if etag != "" && etag == req.Header.Get("If-None-Match") {
		return true
	}

	lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		return false
	}
	ifModSince, err := time.Parse(time.RFC1123, req.Header.Get("If-Modified-Since"))
	if err != nil {
		return false
	}
	if lastModified.Before(ifModSince) || lastModified.Equal(ifModSince) {
		return true
	}

	return false
}

// TransformingTransport is an implementation of http.RoundTripper that
// optionally transforms images using the options specified in the request URL
// fragment.
type TransformingTransport struct {
	// Transport is the underlying http.RoundTripper used to satisfy
	// non-transform requests (those that do not include a URL fragment).
	Transport http.RoundTripper

	// CachingClient is used to fetch images to be resized.  This client is
	// used rather than Transport directly in order to ensure that
	// responses are properly cached.
	CachingClient *http.Client

	logger *zap.SugaredLogger

	pool chan bool
}

// RoundTrip implements the http.RoundTripper interface.
func (t *TransformingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var err error
	if req.URL.Fragment == "" {
		// normal requests pass through, image transformations are signaled in Fragment at this point
		t.logger.Debugw("Fetching remote URL",
			"req.URL", req.URL,
		)

		return t.Transport.RoundTrip(req)
	}

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

	resp, err := t.CachingClient.Get(u.String())
	if err != nil {
		t.logger.Warnw("CachingClient returned an error",
			"u", u,
			"error", err.Error(),
		)
		return nil, err
	}
	defer resp.Body.Close()

	// This should have the side effect of not caching errors
	if resp.StatusCode >= 400 {
		t.logger.Warnw("Erroneous status code, bail out asap",
			"resp.StatusCode", resp.StatusCode,
		)
		return nil, fmt.Errorf("unexpected status code %d from upstream", resp.StatusCode)
	}

	if should304(req, resp) {
		// bare 304 response, full response will be used from cache
		return &http.Response{StatusCode: http.StatusNotModified}, nil
	}

	/*
		Limit concurrency of memory-intensive operations. Writing to a channel with a full buffer blocks...
		so the following will limit the concurrency based on the buffer pool size.

		Note that if the channel is nil this will deadlock, so the nil check is absolutely mandatory.
	*/
	if t.pool != nil {
		t.pool <- true
		defer func() { <-t.pool }() // unblock one writer eventually
	} else {
		t.logger.Infow("t.pool is nil, bad initialization?")
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.logger.Warnw("Error reading a response",
			"u", u,
			"error", err.Error(),
		)
		return nil, err
	}

	err = req.ParseForm()
	if err != nil {
		t.logger.Warnw("Error parsing query string",
			"u", u,
			"error", err.Error(),
		)
		return nil, err
	}

	opt := ParseOptions(req.URL.Fragment)

	t.logger.Infow("Calling Transform",
		"options fragment", req.URL.Fragment,
	)

	img, err := Transform(b, opt)
	if err != nil {
		t.logger.Warnw("Error transforming image",
			"error", err.Error(),
			"opt", opt,
		)
		img = b
	}

	// replay response with transformed image and updated content length
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "%s %s\n", resp.Proto, resp.Status)
	resp.Header.WriteSubset(buf, map[string]bool{
		"Content-Length": true,
		// exclude Content-Type header if the format may have changed during transformation
		"Content-Type": opt.Format != "" || resp.Header.Get("Content-Type") == "image/webp" || resp.Header.Get("Content-Type") == "image/tiff",
	})
	fmt.Fprintf(buf, "Content-Length: %d\n\n", len(img))
	buf.Write(img)

	return http.ReadResponse(bufio.NewReader(buf), req)
}
