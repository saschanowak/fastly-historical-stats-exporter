// Package filter provides regex-based allow/block filtering for metric names.
package filter

import (
	"fmt"
	"regexp"
)

// Filter applies regex-based allow and block rules to strings.
// The zero value permits everything.
// Allowlist patterns are OR'd: a string passes if it matches any pattern.
// Blocklist patterns are OR'd: a string is blocked if it matches any pattern.
// Both lists are AND'd: a string must pass the allowlist AND not be blocked.
type Filter struct {
	allowlist []*regexp.Regexp
	blocklist []*regexp.Regexp
}

// Allow adds a compiled allowlist pattern. Returns an error if expr is invalid.
func (f *Filter) Allow(expr string) error {
	re, err := regexp.Compile(expr)
	if err != nil {
		return fmt.Errorf("invalid allowlist pattern %q: %w", expr, err)
	}
	f.allowlist = append(f.allowlist, re)
	return nil
}

// Block adds a compiled blocklist pattern. Returns an error if expr is invalid.
func (f *Filter) Block(expr string) error {
	re, err := regexp.Compile(expr)
	if err != nil {
		return fmt.Errorf("invalid blocklist pattern %q: %w", expr, err)
	}
	f.blocklist = append(f.blocklist, re)
	return nil
}

// Permit returns true if s passes both allowlist and blocklist checks.
func (f *Filter) Permit(s string) bool {
	for _, re := range f.blocklist {
		if re.MatchString(s) {
			return false
		}
	}
	if len(f.allowlist) == 0 {
		return true
	}
	for _, re := range f.allowlist {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
