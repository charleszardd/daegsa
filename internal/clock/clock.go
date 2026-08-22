package clock

import "time"

// Clock abstracts time operations to enable deterministic timing in tests (§7, §15).
type Clock interface {
	// Now returns the current monotonic time.
	Now() time.Time

	// Since returns the elapsed time since t.
	Since(t time.Time) time.Duration

	// Sleep pauses the current goroutine for at least duration d.
	Sleep(d time.Duration)

	// After returns a channel that receives the current time after duration d.
	After(d time.Duration) <-chan time.Time

	// NewTimer creates a new Timer that will send the current time on its channel after duration d.
	NewTimer(d time.Duration) Timer

	// NewTicker creates a new Ticker that will send the time on its channel with a period of duration d.
	NewTicker(d time.Duration) Ticker
}

// Timer represents an active, cancelable countdown timer.
type Timer interface {
	// C returns the channel on which the timer value is sent.
	C() <-chan time.Time

	// Stop prevents the Timer from firing. It returns true if the call stops the timer,
	// false if the timer has already expired or been stopped.
	Stop() bool

	// Reset changes the timer to expire after duration d.
	// It returns true if the timer had been active, false if the timer had expired or been stopped.
	Reset(d time.Duration) bool
}

// Ticker represents a recurring periodic time event generator.
type Ticker interface {
	// C returns the channel on which ticks are delivered.
	C() <-chan time.Time

	// Stop turns off a ticker. After Stop, no more ticks will be sent.
	Stop()

	// Reset stops a ticker and resets its period to the specified duration.
	Reset(d time.Duration)
}

// RealClock implements Clock using the standard Go time package.
type RealClock struct{}

// NewRealClock returns a Clock backed by the standard library monotonic time.
func NewRealClock() Clock {
	return RealClock{}
}

func (RealClock) Now() time.Time {
	return time.Now()
}

func (RealClock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func (RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{timer: time.NewTimer(d)}
}

func (RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{ticker: time.NewTicker(d)}
}

type realTimer struct {
	timer *time.Timer
}

func (r *realTimer) C() <-chan time.Time {
	return r.timer.C
}

func (r *realTimer) Stop() bool {
	return r.timer.Stop()
}

func (r *realTimer) Reset(d time.Duration) bool {
	return r.timer.Reset(d)
}

type realTicker struct {
	ticker *time.Ticker
}

func (r *realTicker) C() <-chan time.Time {
	return r.ticker.C
}

func (r *realTicker) Stop() {
	r.ticker.Stop()
}

func (r *realTicker) Reset(d time.Duration) {
	r.ticker.Reset(d)
}
