package event

import "testing"

func TestSlowSubscriberIsDroppedNotBlocking(t *testing.T) {
	b := NewBus()
	slow, ok := b.Subscribe(), b.Subscribe()
	for range 64 {
		b.Publish(Event{Topic: "scores"})
		<-ok.C // ok keeps up
	}
	b.Publish(Event{Topic: "scores"}) // slow's buffer is full here
	select {
	case <-slow.Lost:
	default:
		t.Fatal("slow subscriber not dropped")
	}
	select {
	case <-ok.Lost:
		t.Fatal("healthy subscriber dropped")
	default:
	}
	if len(ok.C) != 1 {
		t.Fatal("healthy subscriber missed the event")
	}
}
