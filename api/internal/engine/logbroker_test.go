package engine_test

import (
	"testing"

	"github.com/seantiz/vulcan/internal/engine"
)

func TestLogBrokerSingleSubscriber(t *testing.T) {
	b := engine.NewLogBroker()
	ch, unsub := b.Subscribe("w1")
	defer unsub()

	lines := []string{"line 1", "line 2", "line 3"}
	for _, l := range lines {
		b.Publish("w1", l)
	}
	b.Close("w1")

	var got []string
	for l := range ch {
		got = append(got, l)
	}

	if len(got) != len(lines) {
		t.Fatalf("got %d lines, want %d", len(got), len(lines))
	}
	for i, l := range got {
		if l != lines[i] {
			t.Errorf("line[%d] = %q, want %q", i, l, lines[i])
		}
	}
}

func TestLogBrokerMultipleSubscribers(t *testing.T) {
	b := engine.NewLogBroker()
	ch1, unsub1 := b.Subscribe("w1")
	defer unsub1()
	ch2, unsub2 := b.Subscribe("w1")
	defer unsub2()

	b.Publish("w1", "hello")
	b.Close("w1")

	var got1, got2 []string
	for l := range ch1 {
		got1 = append(got1, l)
	}
	for l := range ch2 {
		got2 = append(got2, l)
	}

	if len(got1) != 1 || got1[0] != "hello" {
		t.Errorf("subscriber 1 got %v, want [hello]", got1)
	}
	if len(got2) != 1 || got2[0] != "hello" {
		t.Errorf("subscriber 2 got %v, want [hello]", got2)
	}
}

func TestLogBrokerCloseClosesChannels(t *testing.T) {
	b := engine.NewLogBroker()
	ch, unsub := b.Subscribe("w1")
	defer unsub()

	b.Close("w1")

	// Channel should be closed; reading should return zero value immediately.
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after Close()")
	}
}

func TestLogBrokerLateSubscriberGetsClosed(t *testing.T) {
	b := engine.NewLogBroker()
	b.Publish("w1", "early")
	b.Close("w1")

	// Subscribe after Close — should get a closed channel.
	ch, unsub := b.Subscribe("w1")
	defer unsub()

	_, ok := <-ch
	if ok {
		t.Error("late subscriber should get a closed channel")
	}
}

func TestLogBrokerUnsubscribeStopsDelivery(t *testing.T) {
	b := engine.NewLogBroker()
	ch, unsub := b.Subscribe("w1")
	unsub()

	b.Publish("w1", "after unsub")
	b.Close("w1")

	// The channel should have no messages (we unsubscribed before publish).
	select {
	case l, ok := <-ch:
		if ok {
			t.Errorf("got unexpected line %q after unsubscribe", l)
		}
	default:
		// No data — expected.
	}
}

func TestLogBrokerPublishToUnknownWorkloadIsNoop(t *testing.T) {
	b := engine.NewLogBroker()
	// Should not panic.
	b.Publish("nonexistent", "line")
	b.Close("nonexistent")
}

func TestLogBrokerLateSubscriberMissesEarlierLines(t *testing.T) {
	b := engine.NewLogBroker()
	ch1, unsub1 := b.Subscribe("w1")
	defer unsub1()

	b.Publish("w1", "line 1")

	// Late subscriber joins after line 1.
	ch2, unsub2 := b.Subscribe("w1")
	defer unsub2()

	b.Publish("w1", "line 2")
	b.Close("w1")

	var got1, got2 []string
	for l := range ch1 {
		got1 = append(got1, l)
	}
	for l := range ch2 {
		got2 = append(got2, l)
	}

	if len(got1) != 2 {
		t.Errorf("subscriber 1 got %d lines, want 2", len(got1))
	}
	if len(got2) != 1 || got2[0] != "line 2" {
		t.Errorf("late subscriber got %v, want [line 2]", got2)
	}
}
