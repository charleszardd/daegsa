package clock_test

import (
	"sync"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
)

func TestRealClock_Basic(t *testing.T) {
	c := clock.NewRealClock()
	now := c.Now()
	if now.IsZero() {
		t.Fatalf("RealClock.Now() returned zero time")
	}

	time.Sleep(1 * time.Millisecond)
	since := c.Since(now)
	if since < 0 {
		t.Errorf("RealClock.Since() = %v, expected >= 0", since)
	}

	timer := c.NewTimer(5 * time.Millisecond)
	select {
	case fireTime := <-timer.C():
		if fireTime.Before(now) {
			t.Errorf("timer fired before now")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timer did not fire in time")
	}

	ticker := c.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	select {
	case <-ticker.C():
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("ticker did not tick in time")
	}
}

func TestControllableClock_Advance(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	timer1 := c.NewTimer(10 * time.Second)
	timer2 := c.NewTimer(20 * time.Second)

	// Advance 5 seconds: neither should fire
	c.Advance(5 * time.Second)
	select {
	case <-timer1.C():
		t.Fatalf("timer1 fired prematurely at 5s")
	case <-timer2.C():
		t.Fatalf("timer2 fired prematurely at 5s")
	default:
	}

	// Advance another 5 seconds (total 10s): timer1 should fire
	c.Advance(5 * time.Second)
	select {
	case fired := <-timer1.C():
		expected := initial.Add(10 * time.Second)
		if !fired.Equal(expected) {
			t.Errorf("timer1 fired at %v, want %v", fired, expected)
		}
	default:
		t.Fatalf("timer1 did not fire at 10s")
	}

	// timer2 should still not have fired
	select {
	case <-timer2.C():
		t.Fatalf("timer2 fired prematurely at 10s")
	default:
	}

	// Advance 10s (total 20s): timer2 should fire
	c.Advance(10 * time.Second)
	select {
	case fired := <-timer2.C():
		expected := initial.Add(20 * time.Second)
		if !fired.Equal(expected) {
			t.Errorf("timer2 fired at %v, want %v", fired, expected)
		}
	default:
		t.Fatalf("timer2 did not fire at 20s")
	}
}

func TestControllableClock_DeterministicOrderAtIdenticalTimestamp(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	var firedOrder []int
	timer1 := c.NewTimer(10 * time.Second)
	timer2 := c.NewTimer(10 * time.Second)
	timer3 := c.NewTimer(10 * time.Second)

	c.Advance(10 * time.Second)

	// Read in sequence
	select {
	case <-timer1.C():
		firedOrder = append(firedOrder, 1)
	default:
	}
	select {
	case <-timer2.C():
		firedOrder = append(firedOrder, 2)
	default:
	}
	select {
	case <-timer3.C():
		firedOrder = append(firedOrder, 3)
	default:
	}

	if len(firedOrder) != 3 || firedOrder[0] != 1 || firedOrder[1] != 2 || firedOrder[2] != 3 {
		t.Errorf("firedOrder = %v, want [1, 2, 3]", firedOrder)
	}
}

func TestControllableClock_TimerStop(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	timer := c.NewTimer(10 * time.Second)
	stopped := timer.Stop()
	if !stopped {
		t.Errorf("expected Stop() to return true for active timer")
	}

	// Advance past timer expiration
	c.Advance(15 * time.Second)

	select {
	case <-timer.C():
		t.Fatalf("stopped timer fired on channel")
	default:
	}

	// Stopping again returns false
	if timer.Stop() {
		t.Errorf("expected second Stop() to return false")
	}
}

func TestControllableClock_TimerReset(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	timer := c.NewTimer(10 * time.Second)
	c.Advance(5 * time.Second)

	// Reset for 10s from now (total 15s from initial)
	wasActive := timer.Reset(10 * time.Second)
	if !wasActive {
		t.Errorf("expected Reset() to report timer was active")
	}

	// Advance 5s (total 10s from initial): should not fire yet
	c.Advance(5 * time.Second)
	select {
	case <-timer.C():
		t.Fatalf("reset timer fired prematurely at 10s")
	default:
	}

	// Advance another 5s (total 15s from initial): should fire
	c.Advance(5 * time.Second)
	select {
	case fired := <-timer.C():
		expected := initial.Add(15 * time.Second)
		if !fired.Equal(expected) {
			t.Errorf("timer fired at %v, want %v", fired, expected)
		}
	default:
		t.Fatalf("reset timer failed to fire at 15s")
	}
}

func TestControllableClock_Ticker(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	ticker := c.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Advance 12 seconds: should tick at 5s and 10s
	c.Advance(5 * time.Second)
	select {
	case <-ticker.C():
	default:
		t.Fatalf("ticker did not tick at 5s")
	}

	c.Advance(5 * time.Second)
	select {
	case <-ticker.C():
	default:
		t.Fatalf("ticker did not tick at 10s")
	}

	ticker.Stop()
	c.Advance(10 * time.Second)
	select {
	case <-ticker.C():
		t.Fatalf("stopped ticker delivered tick")
	default:
	}
}

func TestControllableClock_BlockUntilTimers(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		_ = c.NewTimer(10 * time.Second)
	}()

	// Block until timer is registered
	c.BlockUntilTimers(1)
	if c.ActiveTimersCount() < 1 {
		t.Errorf("ActiveTimersCount() = %d, want at least 1", c.ActiveTimersCount())
	}
	wg.Wait()
}

func TestControllableClock_ConcurrentAccess(t *testing.T) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = c.Now()
				timer := c.NewTimer(time.Duration(j) * time.Millisecond)
				if j%2 == 0 {
					timer.Stop()
				}
			}
		}(i)
	}

	// Concurrently advance time
	for i := 0; i < 50; i++ {
		c.Advance(1 * time.Millisecond)
	}

	wg.Wait()
}
