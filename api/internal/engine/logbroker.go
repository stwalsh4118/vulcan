package engine

import "sync"

// subscriberBufferSize is the channel buffer for each log subscriber.
// Lines are dropped if a subscriber falls this far behind.
const subscriberBufferSize = 64

// LogBroker manages per-workload log streaming to subscribers.
// It is safe for concurrent use.
//
// Closed topics are retained as markers so that late subscribers (those
// subscribing after a workload finishes) receive a closed channel instead of
// blocking forever. Each marker is a few bytes, which is acceptable for the
// expected workload volume.
type LogBroker struct {
	mu     sync.Mutex
	topics map[string]*logTopic
}

type logTopic struct {
	subs   map[int]chan string
	nextID int
	closed bool
}

// NewLogBroker creates a new log broker.
func NewLogBroker() *LogBroker {
	return &LogBroker{
		topics: make(map[string]*logTopic),
	}
}

// Subscribe returns a channel that receives log lines for the given workload
// and an unsubscribe function. If the workload has already finished (Close was
// called), the returned channel is immediately closed.
func (b *LogBroker) Subscribe(workloadID string) (<-chan string, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	t, ok := b.topics[workloadID]
	if !ok {
		t = &logTopic{subs: make(map[int]chan string)}
		b.topics[workloadID] = t
	}

	ch := make(chan string, subscriberBufferSize)
	if t.closed {
		close(ch)
		return ch, func() {}
	}

	id := t.nextID
	t.nextID++
	t.subs[id] = ch

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(t.subs, id)
	}
}

// Publish sends a log line to all subscribers of the given workload.
// Lines are dropped for subscribers whose buffers are full.
func (b *LogBroker) Publish(workloadID string, line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	t, ok := b.topics[workloadID]
	if !ok || t.closed {
		return
	}

	for _, ch := range t.subs {
		select {
		case ch <- line:
		default:
			// Drop line for slow subscribers to avoid blocking execution.
		}
	}
}

// Close signals that no more logs will be published for the given workload.
// All subscriber channels are closed and future Subscribe calls return a
// closed channel.
func (b *LogBroker) Close(workloadID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	t, ok := b.topics[workloadID]
	if !ok {
		// Create a closed marker so late subscribers get a closed channel.
		b.topics[workloadID] = &logTopic{subs: make(map[int]chan string), closed: true}
		return
	}

	t.closed = true
	for id, ch := range t.subs {
		close(ch)
		delete(t.subs, id)
	}
}
