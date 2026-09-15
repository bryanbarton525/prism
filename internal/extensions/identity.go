package extensions

import (
	"fmt"
	"regexp"
	"strings"
)

var standardIdentityPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidateIdentity accepts one portable agent/skill identity that is safe to
// use as both a manifest key and a runtime overlay path component.
func ValidateIdentity(identity string) error {
	if !standardIdentityPattern.MatchString(strings.TrimSpace(identity)) {
		return fmt.Errorf("identity %q must match %s", identity, standardIdentityPattern.String())
	}
	return nil
}
