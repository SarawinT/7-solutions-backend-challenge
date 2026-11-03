package utils

import "time"

// NowUTC returns the current UTC time at millisecond precision, the finest
// resolution MongoDB BSON dates can persist, so timestamps in responses
// match what is later read back.
func NowUTC() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }
