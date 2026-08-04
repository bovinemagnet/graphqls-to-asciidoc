package daemon

import (
	"testing"
	"time"
)

// fakeClock lets the timing rules be tested without sleeping.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) advance(d time.Duration) { c.now = c.now.Add(d) }

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)}
}

func TestNotReadyWithoutAChange(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	clock.advance(time.Hour)
	if d.Ready() {
		t.Fatal("expected no rebuild without a change")
	}
}

func TestNotReadyInsideTheQuietPeriod(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(4 * time.Second)
	if d.Ready() {
		t.Fatal("expected no rebuild four seconds after a change")
	}
}

func TestReadyAfterTheQuietPeriod(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	if !d.Ready() {
		t.Fatal("expected a rebuild five seconds after a change")
	}
}

// A burst of saves keeps resetting the quiet period, so no build happens while
// the user is still typing.
func TestBurstOfChangesResetsTheQuietPeriod(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	for i := 0; i < 10; i++ {
		d.Changed()
		clock.advance(2 * time.Second)
		if d.Ready() {
			t.Fatalf("expected no rebuild during the burst, fired at iteration %d", i)
		}
	}

	clock.advance(3 * time.Second)
	if !d.Ready() {
		t.Fatal("expected a rebuild once the burst settled")
	}
}

// The second condition from the spec: a rebuild may not follow within the
// debounce interval of the previous one. This isolates that condition from
// the quiet-since-change one by making the change settle well before the
// build finishes, so lastChange is already older than the quiet period by
// the time lastBuild is set — only the last-build window can be holding
// Ready() back.
func TestRateLimitedByTheLastBuild(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	d.BuildStarted()

	// A change arrives while the build is running.
	d.Changed()
	clock.advance(3 * time.Second)
	d.BuildCompleted()

	// The change settled two seconds ago; only the build window blocks.
	clock.advance(2 * time.Second)
	if d.Ready() {
		t.Fatal("expected the last build to rate-limit this one")
	}

	clock.advance(3 * time.Second)
	if !d.Ready() {
		t.Fatal("expected a rebuild once the build window passed")
	}
}

func TestChangeDuringABuildIsNotLost(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	d.BuildStarted()

	// The user saves again while the build is running.
	d.Changed()
	d.BuildCompleted()

	clock.advance(10 * time.Second)
	if !d.Ready() {
		t.Fatal("expected the change made during the build to trigger another one")
	}
}

func TestBuildStartedClearsThePendingChange(t *testing.T) {
	clock := newFakeClock()
	d := NewDebouncer(5*time.Second, clock)

	d.Changed()
	clock.advance(5 * time.Second)
	d.BuildStarted()
	d.BuildCompleted()

	clock.advance(time.Hour)
	if d.Ready() {
		t.Fatal("expected no rebuild once the pending change was consumed")
	}
}
