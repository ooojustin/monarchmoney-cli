package config

import (
	"fmt"
	"os"
	"strings"
)

// ResolveSecret expands the "env:NAME" indirection form: a value of "env:NAME"
// is replaced by the value of environment variable NAME, returning an error if
// NAME is unset. Any other value (including the empty string) is returned
// unchanged.
//
// This is the fleet-wide secrets contract: a 0600 file plus environment-variable
// indirection, so an agent can keep the real secret out of the file entirely and
// supply it from the environment at run time.
func ResolveSecret(value string) (string, error) {
	const prefix = "env:"
	if !strings.HasPrefix(value, prefix) {
		return value, nil
	}
	name := strings.TrimPrefix(value, prefix)
	resolved := os.Getenv(name)
	if resolved == "" {
		return "", fmt.Errorf("value references env var %q which is not set", name)
	}
	return resolved, nil
}
