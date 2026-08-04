package daemon

import "time"

// Debouncer decides when a run of filesystem changes has settled enough to be
// worth rebuilding. Two conditions must hold, both taken from the design: the
// quiet period must have elapsed since the last change, and the same interval
// must have elapsed since the last build.
type Debouncer struct {
	quiet      time.Duration
	clock      Clock
	lastChange time.Time
	lastBuild  time.Time
	pending    bool
}

// NewDebouncer returns a debouncer with the given quiet period.
func NewDebouncer(quiet time.Duration, clock Clock) *Debouncer {
	return &Debouncer{quiet: quiet, clock: clock}
}

// Changed records that a watched file was modified.
func (d *Debouncer) Changed() {
	d.lastChange = d.clock.Now()
	d.pending = true
}

// Ready reports whether a pending change has settled and the rate limit allows
// another build.
func (d *Debouncer) Ready() bool {
	if !d.pending {
		return false
	}

	now := d.clock.Now()
	if now.Sub(d.lastChange) < d.quiet {
		return false
	}
	if !d.lastBuild.IsZero() && now.Sub(d.lastBuild) < d.quiet {
		return false
	}
	return true
}

// BuildStarted consumes the pending change. A change arriving during the build
// sets it again, so nothing is lost.
func (d *Debouncer) BuildStarted() {
	d.pending = false
}

// BuildCompleted starts the rate-limit window for the next build.
func (d *Debouncer) BuildCompleted() {
	d.lastBuild = d.clock.Now()
}
