package daemon

import "time"

// Clock is the daemon's view of time. Injecting it keeps the debounce rules
// testable without sleeping.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// SystemClock returns the wall-clock implementation used in production.
func SystemClock() Clock { return systemClock{} }
