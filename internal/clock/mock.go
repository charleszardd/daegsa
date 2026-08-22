package clock

import (
	"container/heap"
	"sync"
	"time"
)

type entryType int

const (
	entryTypeTimer entryType = iota
	entryTypeTicker
)

type mockEntry struct {
	id        uint64
	entryType entryType
	triggerAt time.Time
	period    time.Duration
	ch        chan time.Time
	stopped   bool
	index     int // index in heap
}

type entryHeap []*mockEntry

func (h entryHeap) Len() int { return len(h) }
func (h entryHeap) Less(i, j int) bool {
	if h[i].triggerAt.Equal(h[j].triggerAt) {
		return h[i].id < h[j].id // deterministic sequential ordering
	}
	return h[i].triggerAt.Before(h[j].triggerAt)
}
func (h entryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *entryHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*mockEntry)
	item.index = n
	*h = append(*h, item)
}
func (h *entryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}

// ControllableClock provides a deterministic virtual time clock for testing (§7, §15).
type ControllableClock struct {
	mu      sync.Mutex
	now     time.Time
	entries entryHeap
	nextID  uint64
	waiters []waiter
}

type waiter struct {
	count int
	ch    chan struct{}
}

// NewControllableClock creates a ControllableClock initialized to initialTime.
func NewControllableClock(initialTime time.Time) *ControllableClock {
	c := &ControllableClock{
		now:     initialTime,
		entries: make(entryHeap, 0),
	}
	heap.Init(&c.entries)
	return c
}

// Now returns the current virtual monotonic time.
func (c *ControllableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Since returns the elapsed virtual time since t.
func (c *ControllableClock) Since(t time.Time) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now.Sub(t)
}

// Sleep pauses until virtual time is advanced by at least d.
func (c *ControllableClock) Sleep(d time.Duration) {
	timer := c.NewTimer(d)
	<-timer.C()
}

// After returns a channel that receives virtual time after duration d.
func (c *ControllableClock) After(d time.Duration) <-chan time.Time {
	return c.NewTimer(d).C()
}

// NewTimer creates a mock Timer that will fire after virtual duration d.
func (c *ControllableClock) NewTimer(d time.Duration) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	entry := &mockEntry{
		id:        c.nextID,
		entryType: entryTypeTimer,
		triggerAt: c.now.Add(d),
		period:    d,
		ch:        make(chan time.Time, 1),
	}
	heap.Push(&c.entries, entry)
	c.notifyWaitersLocked()

	return &mockTimer{clock: c, entry: entry}
}

// NewTicker creates a mock Ticker that fires periodically every virtual duration d.
func (c *ControllableClock) NewTicker(d time.Duration) Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	entry := &mockEntry{
		id:        c.nextID,
		entryType: entryTypeTicker,
		triggerAt: c.now.Add(d),
		period:    d,
		ch:        make(chan time.Time, 1),
	}
	heap.Push(&c.entries, entry)
	c.notifyWaitersLocked()

	return &mockTicker{clock: c, entry: entry}
}

// Advance moves virtual time forward by duration d and fires all due timers and tickers in order.
func (c *ControllableClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	targetTime := c.now.Add(d)

	for len(c.entries) > 0 {
		earliest := c.entries[0]
		if earliest.triggerAt.After(targetTime) {
			break
		}

		// Pop earliest entry
		entry := heap.Pop(&c.entries).(*mockEntry)
		c.now = entry.triggerAt

		if !entry.stopped {
			select {
			case entry.ch <- c.now:
			default:
			}

			if entry.entryType == entryTypeTicker && entry.period > 0 {
				entry.triggerAt = entry.triggerAt.Add(entry.period)
				heap.Push(&c.entries, entry)
			}
		}
	}

	c.now = targetTime
}

// Set sets the virtual clock to a specific timestamp.
func (c *ControllableClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// ActiveTimersCount returns the number of active, unexpired/unstopped timers and tickers.
func (c *ControllableClock) ActiveTimersCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, e := range c.entries {
		if !e.stopped {
			count++
		}
	}
	return count
}

// BlockUntilTimers blocks until at least expectedCount timers/tickers are registered.
func (c *ControllableClock) BlockUntilTimers(expectedCount int) {
	c.mu.Lock()
	if c.activeTimersCountLocked() >= expectedCount {
		c.mu.Unlock()
		return
	}

	w := waiter{
		count: expectedCount,
		ch:    make(chan struct{}),
	}
	c.waiters = append(c.waiters, w)
	c.mu.Unlock()

	<-w.ch
}

func (c *ControllableClock) activeTimersCountLocked() int {
	count := 0
	for _, e := range c.entries {
		if !e.stopped {
			count++
		}
	}
	return count
}

func (c *ControllableClock) notifyWaitersLocked() {
	active := c.activeTimersCountLocked()
	remaining := c.waiters[:0]
	for _, w := range c.waiters {
		if active >= w.count {
			close(w.ch)
		} else {
			remaining = append(remaining, w)
		}
	}
	c.waiters = remaining
}

type mockTimer struct {
	clock *ControllableClock
	entry *mockEntry
}

func (m *mockTimer) C() <-chan time.Time {
	return m.entry.ch
}

func (m *mockTimer) Stop() bool {
	m.clock.mu.Lock()
	defer m.clock.mu.Unlock()

	if m.entry.stopped {
		return false
	}
	m.entry.stopped = true
	if m.entry.index >= 0 && m.entry.index < len(m.clock.entries) {
		heap.Remove(&m.clock.entries, m.entry.index)
	}
	return true
}

func (m *mockTimer) Reset(d time.Duration) bool {
	m.clock.mu.Lock()
	defer m.clock.mu.Unlock()

	wasActive := !m.entry.stopped
	m.entry.stopped = false
	m.entry.triggerAt = m.clock.now.Add(d)
	m.entry.period = d

	if m.entry.index >= 0 && m.entry.index < len(m.clock.entries) {
		heap.Fix(&m.clock.entries, m.entry.index)
	} else {
		heap.Push(&m.clock.entries, m.entry)
	}

	return wasActive
}

type mockTicker struct {
	clock *ControllableClock
	entry *mockEntry
}

func (m *mockTicker) C() <-chan time.Time {
	return m.entry.ch
}

func (m *mockTicker) Stop() {
	m.clock.mu.Lock()
	defer m.clock.mu.Unlock()

	m.entry.stopped = true
	if m.entry.index >= 0 && m.entry.index < len(m.clock.entries) {
		heap.Remove(&m.clock.entries, m.entry.index)
	}
}

func (m *mockTicker) Reset(d time.Duration) {
	m.clock.mu.Lock()
	defer m.clock.mu.Unlock()

	m.entry.period = d
	m.entry.triggerAt = m.clock.now.Add(d)
	m.entry.stopped = false

	if m.entry.index >= 0 && m.entry.index < len(m.clock.entries) {
		heap.Fix(&m.clock.entries, m.entry.index)
	} else {
		heap.Push(&m.clock.entries, m.entry)
	}
}
