package farm

import (
	"strconv"
	"strings"
)

// versionETag renders the stored version as an entity tag. Concurrency is
// enforced by the store's compare-and-swap on that same version, so the tag a
// client echoes back in If-Match is the value the write is validated against
// rather than a hash compared in a separate, racy step.
func versionETag(version int64) string {
	return `"v` + strconv.FormatInt(version, 10) + `"`
}

// parseETag recovers the version from an If-Match header, reporting whether the
// header was a tag this server could have issued.
func parseETag(value string) (int64, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(value), "W/")
	if len(trimmed) < 4 || !strings.HasPrefix(trimmed, `"v`) || !strings.HasSuffix(trimmed, `"`) {
		return 0, false
	}
	version, err := strconv.ParseInt(trimmed[2:len(trimmed)-1], 10, 64)
	if err != nil {
		return 0, false
	}
	return version, true
}
