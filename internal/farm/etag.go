package farm

import (
	"crypto/sha256"
	"fmt"
)

func stateETag(state []byte) string {
	sum := sha256.Sum256(state)
	return fmt.Sprintf(`"%x"`, sum[:12])
}
