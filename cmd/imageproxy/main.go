// This command starts an HTTP server that proxies requests for remote images.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PaulARoy/azurestoragecache"
	"github.com/die-net/lrucache"
	"github.com/die-net/lrucache/twotier"
	"github.com/garyburd/redigo/redis"
	"github.com/gregjones/httpcache/diskcache"
	rediscache "github.com/gregjones/httpcache/redis"
	"github.com/peterbourgon/diskv"
	"go.uber.org/zap"

	"github.com/richiefi/imageproxy"
	"github.com/richiefi/imageproxy/internal/gcscache"
	"github.com/richiefi/imageproxy/internal/s3cache"
)

const defaultMemorySize = 100

var addr = flag.String("addr", "localhost:8080", "TCP address to listen on")
var whitelist = flag.String("whitelist", "", "comma separated list of allowed remote hosts")
var referrers = flag.String("referrers", "", "comma separated list of allowed referring hosts")
var baseURLConfURL = flag.String("baseURLConfURL", "", "location of json object of url prefixes for this service")
var cache tieredCache
var signatureKey = flag.String("signatureKey", "", "HMAC key used in calculating request signatures")
var scaleUp = flag.Bool("scaleUp", false, "allow images to scale beyond their original dimensions")
var timeout = flag.Duration("timeout", 0, "time limit for requests served by this proxy")
var verbose = flag.Bool("verbose", false, "print verbose logging messages")
var version = flag.Bool("version", false, "Deprecated: this flag does nothing")
var maxConcurrency = flag.Int("maxConcurrency", 16, "Maximum number of concurrent memory-intensive operations")

func init() {
	flag.Var(&cache, "cache", "location to cache images (see https://github.com/willnorris/imageproxy#cache)")
}

func buildLogger() *zap.SugaredLogger {
	cfg := zap.NewProductionConfig()
	if *verbose {
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	plainLogger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return plainLogger.Sugar()
}

func main() {
	flag.Parse()

	logger := buildLogger()

	p := imageproxy.NewProxy(nil, cache.Cache, *maxConcurrency, logger)

	if *whitelist != "" {
		p.Whitelist = strings.Split(*whitelist, ",")
	}
	if *referrers != "" {
		p.Referrers = strings.Split(*referrers, ",")
	}
	if *signatureKey != "" {
		key := []byte(*signatureKey)
		if strings.HasPrefix(*signatureKey, "@") {
			file := strings.TrimPrefix(*signatureKey, "@")
			var err error
			key, err = ioutil.ReadFile(file)
			if err != nil {
				logger.Fatalw("error reading signature file",
					"signatureKey", signatureKey,
					"error", err.Error(),
				)
			}
		}
		p.SignatureKey = key
	}

	// Empty map as a default, try to fill it
	p.PrefixesToConfigs = make(map[string]*imageproxy.SourceConfiguration, 0)

	if *baseURLConfURL != "" {
		resp, err := http.Get(*baseURLConfURL)
		if err != nil {
			logger.Fatalw("Could not download URL mapping JSON",
				"error", err.Error(),
				"*baseURLConfURL", baseURLConfURL,
			)
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		err = decoder.Decode(&p.PrefixesToConfigs)
		if err != nil {
			logger.Fatalw("Could not read prefix mapping JSON",
				"error", err.Error(),
			)
		}

	}

	p.Timeout = *timeout
	p.ScaleUp = *scaleUp

	if os.Getenv("CASCADE_XML_PATH") == "" {
		logger.Warnw("'CASCADE_XML_PATH' is not set. Face detection disabled.")
	}

	server := &http.Server{
		Addr:    *addr,
		Handler: p,
	}

	logger.Infow("imageproxy listening",
		"server.Addr", server.Addr,
	)
	logger.Fatal(server.ListenAndServe())
}

// tieredCache allows specifying multiple caches via flags, which will create
// tiered caches using the twotier package.
type tieredCache struct {
	imageproxy.Cache
}

func (tc *tieredCache) String() string {
	return fmt.Sprint(*tc)
}

func (tc *tieredCache) Set(value string) error {
	c, err := parseCache(value)
	if err != nil {
		return err
	}

	if tc.Cache == nil {
		tc.Cache = c
	} else {
		tc.Cache = twotier.New(tc.Cache, c)
	}
	return nil
}

// parseCache parses c returns the specified Cache implementation.
func parseCache(c string) (imageproxy.Cache, error) {
	if c == "" {
		return nil, nil
	}

	if c == "memory" {
		c = fmt.Sprintf("memory:%d", defaultMemorySize)
	}

	u, err := url.Parse(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing cache flag: %v", err)
	}

	switch u.Scheme {
	case "azure":
		return azurestoragecache.New("", "", u.Host)
	case "gcs":
		return gcscache.New(u.Host, strings.TrimPrefix(u.Path, "/"))
	case "memory":
		return lruCache(u.Opaque)
	case "redis":
		conn, err := redis.DialURL(u.String(), redis.DialPassword(os.Getenv("REDIS_PASSWORD")))
		if err != nil {
			return nil, err
		}
		return rediscache.NewWithClient(conn), nil
	case "s3":
		return s3cache.New(u.String())
	case "file":
		fallthrough
	default:
		return diskCache(u.Path), nil
	}
}

// lruCache creates an LRU Cache with the specified options of the form
// "maxSize:maxAge".  maxSize is specified in megabytes, maxAge is a duration.
func lruCache(options string) (*lrucache.LruCache, error) {
	parts := strings.SplitN(options, ":", 2)
	size, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}

	var age time.Duration
	if len(parts) > 1 {
		age, err = time.ParseDuration(parts[1])
		if err != nil {
			return nil, err
		}
	}

	return lrucache.New(size*1e6, int64(age.Seconds())), nil
}

func diskCache(path string) *diskcache.Cache {
	d := diskv.New(diskv.Options{
		BasePath: path,

		// For file "c0ffee", store file as "c0/ff/c0ffee"
		Transform: func(s string) []string { return []string{s[0:2], s[2:4]} },
	})
	return diskcache.NewWithDiskv(d)
}
