package safelog

import "strings"

const resetPrefix = "/reset/"

func URI(uri string) string {
	if i := strings.Index(uri, "?"); i >= 0 {
		uri = uri[:i]
	}

	if strings.HasPrefix(uri, resetPrefix) && len(uri) > len(resetPrefix) {
		return resetPrefix + "[redacted]"
	}

	return uri
}
