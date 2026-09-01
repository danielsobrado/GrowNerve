package farm

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
)

const farmVersionHeader = "X-Farm-Version"

// representationETag is a strong validator for the actual response body. The
// state response projects live telemetry, so its ETag must change when that
// projection changes even if the configuration version does not.
func representationETag(body []byte) string {
	digest := sha256.Sum256(body)
	return fmt.Sprintf(`"%x"`, digest)
}

func versionHeader(version int64) string {
	return strconv.FormatInt(version, 10)
}

func parseVersionHeader(value string) (int64, bool) {
	version, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || version < 0 {
		return 0, false
	}
	return version, true
}

// versionETag and parseETag remain for compatibility with older clients. New
// clients use X-Farm-Version so HTTP representation caching is not conflated
// with optimistic-concurrency state versioning.
func versionETag(version int64) string {
	return `"v` + strconv.FormatInt(version, 10) + `"`
}

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
