package options_test

import (
	"net/url"
	"testing"

	"github.com/richiefi/imageproxy/options"
)

var emptyOptions = options.Options{}

func TestOptions_String(t *testing.T) {
	tests := []struct {
		Options options.Options
		String  string
	}{
		{
			emptyOptions,
			"0x0",
		},
		{
			options.Options{1, 2, true, 90, true, true, 80, "", false, "", 0, 0, 0, 0, false},
			"1x2,fit,r90,fv,fh,q80",
		},
		{
			options.Options{0.15, 1.3, false, 45, false, false, 95, "c0ffee", false, "png", 0, 0, 0, 0, false},
			"0.15x1.3,r45,q95,sc0ffee,png",
		},
		{
			options.Options{0.15, 1.3, false, 45, false, false, 95, "c0ffee", false, "", 100, 200, 0, 0, false},
			"0.15x1.3,r45,q95,sc0ffee,cx100,cy200",
		},
		{
			options.Options{0.15, 1.3, false, 45, false, false, 95, "c0ffee", false, "png", 100, 200, 300, 400, false},
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
		Options options.Options
	}{
		{"", emptyOptions},
		{"x", emptyOptions},
		{"r", emptyOptions},
		{"0", emptyOptions},
		{"crop=,,,,", emptyOptions},

		// size variations
		{"width=1", options.Options{Width: 1}},
		{"height=1", options.Options{Height: 1}},
		{"width=1&height=2", options.Options{Width: 1, Height: 2, Fit: true}},
		{"width=-1&height=-2", options.Options{Width: -1, Height: -2}},
		{"width=0.1&height=0.2", options.Options{Width: 0.1, Height: 0.2, Fit: true}},
		{"size=1", options.Options{Width: 1, Height: 1, Fit: true}},
		{"size=0.1", options.Options{Width: 0.1, Height: 0.1, Fit: true}},

		// sizes with dpr
		{"width=1&dpr=3", options.Options{Width: 3}},
		{"height=1&dpr=3", options.Options{Height: 3}},
		{"width=1&height=2&dpr=3", options.Options{Width: 3, Height: 6, Fit: true}},
		{"width=-1&height=-2&dpr=3", options.Options{Width: -3, Height: -6}},
		{"width=0.1&height=0.2&dpr=3", options.Options{Width: 0.3, Height: 0.6, Fit: true}},
		{"size=1&dpr=3", options.Options{Width: 3, Height: 3, Fit: true}},
		{"size=0.1&dpr=3", options.Options{Width: 0.3, Height: 0.3, Fit: true}},

		// crop is smart
		{"mode=crop&size=200", options.Options{Width: 200, Height: 200, SmartCrop: true}},
		{"mode=smartcrop&size=200", options.Options{Width: 200, Height: 200, SmartCrop: true}},

		// additional flags
		{"mode=fit", options.Options{Fit: true}},
		{"rotate=90", options.Options{Rotate: 90}},
		{"flip=v", options.Options{FlipVertical: true}},
		{"flip=h", options.Options{FlipHorizontal: true}},
		{"format=jpeg", options.Options{Format: "jpeg"}},

		// mix of valid and invalid flags
		{"FOO=BAR&size=1&BAR=foo&rotate=90&BAZ=DAS", options.Options{Width: 1, Height: 1, Rotate: 90, Fit: true}},

		// flags, in different orders
		{"quality=70&width=1&height=2&mode=fit&rotate=90&flip=v&flip=h&signature=c0ffee&format=png", options.Options{1, 2, true, 90, true, true, 70, "c0ffee", false, "png", 0, 0, 0, 0, false}},
		{"rotate=90&flip=h&signature=c0ffee&format=png&quality=90&width=1&height=2&flip=v&mode=fit", options.Options{1, 2, true, 90, true, true, 90, "c0ffee", false, "png", 0, 0, 0, 0, false}},

		// all flags, in different orders with crop
		{"quality=70&width=1&height=2&mode=fit&crop=100,200,300,400&rotate=90&flip=v&flip=h&signature=c0ffee&format=png", options.Options{1, 2, true, 90, true, true, 70, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"rotate=90&flip=h&signature=c0ffee&format=png&crop=100,200,300,400&quality=90&width=1&height=2&flip=v&mode=fit", options.Options{1, 2, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},

		// all flags, in different orders with crop & different resizes
		{"quality=70&crop=100,200,300,400&height=2&mode=fit&rotate=90&flip=v&flip=h&signature=c0ffee&format=png", options.Options{0, 2, true, 90, true, true, 70, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"crop=100,200,300,400&rotate=90&flip=h&quality=90&signature=c0ffee&format=png&width=1&flip=v&mode=fit", options.Options{1, 0, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"crop=100,200,300,400&rotate=90&flip=h&signature=c0ffee&flip=v&format=png&quality=90&mode=fit", options.Options{0, 0, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},
		{"crop=100,200,0,400&rotate=90&quality=90&flip=h&signature=c0ffee&format=png&flip=v&mode=fit&width=123&height=321", options.Options{123, 321, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 0, 400, false}},
		{"flip=v&width=123&height=321&crop=100,200,300,400&quality=90&rotate=90&flip=h&signature=c0ffee&format=png&mode=fit", options.Options{123, 321, true, 90, true, true, 90, "c0ffee", false, "png", 100, 200, 300, 400, false}},
	}

	for _, tt := range tests {
		input, err := url.ParseQuery(tt.InputQS)
		if err != nil {
			panic(err)
		}

		if got, want := options.ParseFormValues(input, options.Options{}), tt.Options; !got.Equal(want) {
			t.Errorf("ParseFormValues(%q) returned %#v, want %#v", tt.InputQS, got, want)
		}
	}
}
