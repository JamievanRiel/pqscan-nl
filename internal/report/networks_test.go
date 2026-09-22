package report

import "testing"

func TestNetworkName(t *testing.T) {
	for in, want := range map[string]string{
		"CLOUDFLARENET":                         "Cloudflare",
		"TRANSIP-AS Amsterdam, the Netherlands": "TransIP",
		"EXAMPLE-AS Example BV":                 "EXAMPLE-AS Example BV",
		"":                                      "",
	} {
		if got := NetworkName(in); got != want {
			t.Errorf("NetworkName(%q) = %q, want %q", in, got, want)
		}
	}
}
