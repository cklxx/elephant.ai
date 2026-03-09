package hooks

import (
	"testing"
	"time"
)

func TestInProcessBus_PublishAndSubscribe(t *testing.T) {
	bus := NewInProcessBus()

	ch, cancel := bus.Subscribe("session-1")
	defer cancel()

	ev := Event{Type: EventHeartbeat, SessionID: "session-1", At: time.Now()}
	bus.Publish("session-1", ev)

	select {
	case got := <-ch:
		if got.Type != EventHeartbeat || got.SessionID != "session-1" {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestInProcessBus_SubscribeAll_FanOut(t *testing.T) {
	bus := NewInProcessBus()

	chAll, cancelAll := bus.SubscribeAll()
	defer cancelAll()

	chSess, cancelSess := bus.Subscribe("s1")
	defer cancelSess()

	ev := Event{Type: EventCompleted, SessionID: "s1", At: time.Now()}
	bus.Publish("s1", ev)

	// Both the specific and wildcard subscriber should receive the event.
	for _, ch := range []<-chan Event{chAll, chSess} {
		select {
		case got := <-ch:
			if got.Type != EventCompleted {
				t.Fatalf("unexpected type: %s", got.Type)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}
}

func TestInProcessBus_WrongSession_NoDelivery(t *testing.T) {
	bus := NewInProcessBus()

	ch, cancel := bus.Subscribe("session-A")
	defer cancel()

	// Publish to a different session.
	bus.Publish("session-B", Event{Type: EventFailed, SessionID: "session-B", At: time.Now()})

	select {
	case ev := <-ch:
		t.Fatalf("should not have received event for other session: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestInProcessBus_Cancel(t *testing.T) {
	bus := NewInProcessBus()

	ch, cancel := bus.Subscribe("s")
	cancel() // unsubscribe immediately

	bus.Publish("s", Event{Type: EventHeartbeat, SessionID: "s", At: time.Now()})

	select {
	case ev := <-ch:
		t.Fatalf("should not receive event after cancel: %+v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestInProcessBus_MultipleSubscribers(t *testing.T) {
	bus := NewInProcessBus()

	ch1, cancel1 := bus.Subscribe("s")
	defer cancel1()
	ch2, cancel2 := bus.Subscribe("s")
	defer cancel2()

	bus.Publish("s", Event{Type: EventStarted, SessionID: "s", At: time.Now()})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Type != EventStarted {
				t.Fatalf("sub %d: unexpected type %s", i, got.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("sub %d: timed out", i)
		}
	}
}
