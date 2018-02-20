package imageproxy

import "testing"

func TestNopCache(t *testing.T) {
	data, ok := NopCache.Get("foo")
	if data != nil {
		t.Errorf("NopCache.Get returned non-nil data")
	}
	if ok != false {
		t.Errorf("NopCache.Get returned ok = true, should always be false.")
	}
}
