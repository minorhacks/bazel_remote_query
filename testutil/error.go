package testutil

import (
	"fmt"
	"strings"
)

func ErrSubstring(got error, want string) string {
	if want == "" {
		if got == nil {
			return ""
		}
		return fmt.Sprintf("got err=%v, want err=nil", got)
	}
	if got == nil {
		return fmt.Sprintf("got err=nil, want err containing %q", want)
	}
	if !strings.Contains(got.Error(), want) {
		return fmt.Sprintf("got err=%v, want err containing %q", got, want)
	}
	return ""
}
