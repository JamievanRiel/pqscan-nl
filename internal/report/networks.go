package report

import "strings"

// networkNames maps the first word of an AS name, the network's handle, to
// the name of the organisation behind it.
var networkNames = map[string]string{
	"AKAMAI-AS":                   "Akamai",
	"AKAMAI-ASN1":                 "Akamai",
	"AMAZON-02":                   "Amazon Web Services",
	"AMAZON-AES":                  "Amazon Web Services",
	"CLDIN-NL":                    "Your Hosting",
	"CLOUDFLARENET":               "Cloudflare",
	"COMBELL-AS":                  "Combell",
	"DIGITALOCEAN-ASN":            "DigitalOcean",
	"DUOCAST-AS":                  "Duocast",
	"FASTLY":                      "Fastly",
	"GOOGLE":                      "Google",
	"GOOGLE-CLOUD-PLATFORM":       "Google Cloud",
	"HETZNER-AS":                  "Hetzner",
	"LEASEWEB-NL-AMS-01":          "Leaseweb",
	"MICROSOFT-CORP-MSN-AS-BLOCK": "Microsoft",
	"NL-BIT":                      "BIT",
	"OVH":                         "OVHcloud",
	"PREVIDER-AS":                 "Previder",
	"PROLOCATION":                 "Prolocation",
	"SURFNET-NL":                  "SURF",
	"TRANSIP-AS":                  "TransIP",
	"TRUESERVER-AS":               "TrueServer",
}

// NetworkName returns a readable name for an AS name from the iptoasn table:
// "TRANSIP-AS Amsterdam, the Netherlands" becomes "TransIP". Unknown networks
// keep their AS name.
func NetworkName(asOrg string) string {
	handle, _, _ := strings.Cut(asOrg, " ")
	if name, ok := networkNames[handle]; ok {
		return name
	}
	return asOrg
}
