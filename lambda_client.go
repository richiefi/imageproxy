package imageproxy

import "net/url"

type LambdaTransformRequest struct {
	URLString string  `json:"u"`
	Options   Options `json:"o"`
}

type LambdaTransformResponse struct {
	Status int    `json:"s"`
	Image  []byte `json:"i"`
}

func TransformWithURL(u *url.URL) (int, []byte, error) {
	// TransformWithURL will call Lambda synchronously and executes DoTransformWithURL there
	return 0, nil, nil
}
