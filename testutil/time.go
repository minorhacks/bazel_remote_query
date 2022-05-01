package testutil

import (
	"fmt"
	"time"
)

func StaticTimeRFC3339(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		panic(fmt.Sprintf("invalid time %q: %v", timeStr, err))
	}
	return t
}
