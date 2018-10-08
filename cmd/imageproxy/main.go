// This command starts an HTTP server that proxies requests for remote images.
package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"

	"github.com/richiefi/imageproxy"
)

const defaultMemorySize = 100

var addr = flag.String("addr", "localhost:8080", "TCP address to listen on")
var lambdaFunctionName = flag.String("lambdaFunctionName", "", "Transforming Lambda function name")
var whitelist = flag.String("whitelist", "", "comma separated list of allowed remote hosts")
var referrers = flag.String("referrers", "", "comma separated list of allowed referring hosts")
var baseURLConfURL = flag.String("baseURLConfURL", "", "location of json object of url prefixes for this service")
var signatureKey = flag.String("signatureKey", "", "HMAC key used in calculating request signatures")
var scaleUp = flag.Bool("scaleUp", false, "allow images to scale beyond their original dimensions")
var timeout = flag.Duration("timeout", 0, "time limit for requests served by this proxy")
var verbose = flag.Bool("verbose", false, "print verbose logging messages")
var version = flag.Bool("version", false, "Deprecated: this flag does nothing")

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

	if os.Getenv("SENTRY_DSN") == "" {
		logger.Warnw("SENTRY_DSN is not set")
	}

	if lambdaFunctionName == nil || *lambdaFunctionName == "" {
		logger.Fatalw("Flag lambdaFunctionName not set. This version will not work without it.")
	}

	p := imageproxy.NewProxy(nil, *lambdaFunctionName, logger)

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

	server := &http.Server{
		Addr:    *addr,
		Handler: p,
	}

	logger.Infow("imageproxy listening",
		"server.Addr", server.Addr,
	)
	logger.Fatal(server.ListenAndServe())
}
