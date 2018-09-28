package lambda

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/richiefi/imageproxy/options"
	"github.com/richiefi/imageproxy/transform"
)

type LambdaExecutor interface {
	DoTransformWithURL(string, options.Options) (int, []byte, error)
}

type lambdaExecutor struct {
	logger     *zap.SugaredLogger
	httpClient *http.Client
}

func NewLambdaExecutor(logger *zap.SugaredLogger) (LambdaExecutor, error) {
	return &lambdaExecutor{
		logger: logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (ex *lambdaExecutor) DoTransformWithURL(u string, options options.Options) (int, []byte, error) {
	logctx := ex.logger.With(
		"func", "DoTransformWithURL",
		"u", u,
		"options", options,
	)

	then := time.Now()

	resp, err := ex.httpClient.Get(u)
	if err != nil {
		logctx.Warnw("Performing upstream HTTP request failed",
			"Error", err.Error(),
		)
		return http.StatusInternalServerError, nil, err
	}
	defer resp.Body.Close()

	logctx.Infow("Performed an HTTP request",
		"duration", time.Since(then),
	)

	then = time.Now()

	if resp.StatusCode >= 400 {
		logctx.Warnw("Got HTTP response with an erroneous response code",
			"StatusCode", resp.StatusCode,
		)
		return resp.StatusCode, nil, fmt.Errorf("HTTP %d from upstream", resp.StatusCode)
	}

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logctx.Warnw("Could not read the body",
			"Error", err.Error(),
		)
		return http.StatusInternalServerError, nil, fmt.Errorf("Error reading body: %s", err.Error())
	}

	logctx.Infow("Read the body",
		"duration", time.Since(then),
	)

	then = time.Now()

	img, err := transform.Transform(bs, options)
	if err != nil {
		logctx.Warnw("Could not transform",
			"Error", err.Error(),
		)
		return http.StatusInternalServerError, bs, fmt.Errorf("Error transforming: %s", err.Error())
	}

	logctx.Infow("Transformed",
		"duration", time.Since(then),
	)

	return http.StatusOK, img, nil
}
