//go:build !minimal

package telegram_test

import (
	"context"
	telegramservice "github.com/MalenkiySolovey/solovey-component-telegram/service"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTelegramNotifierRetriesAndAuditsFailure(t *testing.T) {
	type auditRecord struct {
		event   string
		details map[string]any
	}
	auditCh := make(chan auditRecord, 1)
	var attempts atomic.Int32
	notifier := telegramservice.NewNotifier(4, func(string) telegramservice.Result {
		attempts.Add(1)
		return telegramservice.Result{ErrorClass: "network"}
	}, func(event string, details map[string]any) {
		auditCh <- auditRecord{event: event, details: details}
	})
	notifier.Backoff = []time.Duration{time.Millisecond, time.Millisecond}

	notifier.Enqueue(telegramservice.Notification{Event: "login_failed", Text: "message body"})

	var record auditRecord
	select {
	case record = <-auditCh:
	case <-time.After(time.Second):
		t.Fatal("notifier_failed audit was not recorded")
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
	if record.event != "notifier_failed" {
		t.Fatalf("unexpected audit event: %s", record.event)
	}
	if record.details["channel"] != "telegram" ||
		record.details["event"] != "login_failed" ||
		record.details["errorClass"] != "network" ||
		record.details["attempts"] != 3 {
		t.Fatalf("unexpected audit details: %#v", record.details)
	}
	if _, ok := record.details["text"]; ok {
		t.Fatalf("message text leaked to audit details: %#v", record.details)
	}
}

func TestTelegramNotifierStopCancelsBackoff(t *testing.T) {
	firstSend := make(chan struct{})
	var attempts atomic.Int32
	var firstOnce sync.Once
	notifier := telegramservice.NewNotifier(1, func(string) telegramservice.Result {
		if attempts.Add(1) == 1 {
			firstOnce.Do(func() { close(firstSend) })
		}
		return telegramservice.Result{ErrorClass: "network"}
	}, nil)
	notifier.Backoff = []time.Duration{time.Hour}

	notifier.Enqueue(telegramservice.Notification{Event: "backoff-cancellation", Text: "message"})
	waitForTestChannel(t, firstSend, time.Second, "first telegram send did not start")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := notifier.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Stop did not cancel notifier backoff quickly; elapsed=%s", elapsed)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("Stop should prevent a retry after backoff cancellation, got %d sends", got)
	}
}

func TestTelegramNotifierStopCancelsInFlightSend(t *testing.T) {
	sendStarted := make(chan struct{})
	sendCanceled := make(chan struct{})
	notifier := telegramservice.NewNotifierContext(1, func(ctx context.Context, _ string) telegramservice.Result {
		close(sendStarted)
		<-ctx.Done()
		close(sendCanceled)
		return telegramservice.Result{ErrorClass: "network"}
	}, nil)
	notifier.Backoff = nil
	notifier.Enqueue(telegramservice.Notification{Event: "shutdown", Text: "message"})
	waitForTestChannel(t, sendStarted, time.Second, "telegram send did not start")

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := notifier.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	waitForTestChannel(t, sendCanceled, time.Second, "telegram send did not observe lifecycle cancellation")
}

func TestTelegramNotifierStopDiscardsQueuedWork(t *testing.T) {
	firstStarted := make(chan struct{})
	var delivered atomic.Int32
	notifier := telegramservice.NewNotifierContext(4, func(ctx context.Context, text string) telegramservice.Result {
		if delivered.Add(1) == 1 {
			close(firstStarted)
			<-ctx.Done()
		}
		return telegramservice.Result{ErrorClass: "network"}
	}, nil)
	notifier.Backoff = nil
	notifier.Enqueue(telegramservice.Notification{Event: "first", Text: "first"})
	waitForTestChannel(t, firstStarted, time.Second, "first telegram send did not start")
	notifier.Enqueue(telegramservice.Notification{Event: "queued", Text: "queued"})

	if err := notifier.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := delivered.Load(); got != 1 {
		t.Fatalf("Stop delivered queued work after cancellation: sends=%d", got)
	}
	if got := notifier.QueueLen(); got != 0 {
		t.Fatalf("Stop retained %d queued notifications", got)
	}
}

func TestTelegramNotifierDropsOldestAndAuditsOverflow(t *testing.T) {
	type auditRecord struct {
		event   string
		details map[string]any
	}
	auditCh := make(chan auditRecord, 1)
	sent := make(chan string, 4)
	releaseFirst := make(chan struct{})
	var blockFirst sync.Once
	notifier := telegramservice.NewNotifier(2, func(text string) telegramservice.Result {
		sent <- text
		blockFirst.Do(func() { <-releaseFirst })
		return telegramservice.Result{Success: true}
	}, func(event string, details map[string]any) {
		auditCh <- auditRecord{event: event, details: details}
	})
	notifier.Backoff = nil

	notifier.Enqueue(telegramservice.Notification{Event: "e1", Text: "e1"})
	if got := receiveString(t, sent, "first send"); got != "e1" {
		t.Fatalf("unexpected first send: %s", got)
	}
	notifier.Enqueue(telegramservice.Notification{Event: "e2", Text: "e2"})
	notifier.Enqueue(telegramservice.Notification{Event: "e3", Text: "e3"})
	notifier.Enqueue(telegramservice.Notification{Event: "e4", Text: "e4"})

	var record auditRecord
	select {
	case record = <-auditCh:
	case <-time.After(time.Second):
		t.Fatal("notifier_overflow audit was not recorded")
	}
	if record.event != "notifier_overflow" {
		t.Fatalf("unexpected audit event: %s", record.event)
	}
	if record.details["channel"] != "telegram" ||
		record.details["droppedEvent"] != "e2" ||
		record.details["queuedEvent"] != "e4" {
		t.Fatalf("unexpected overflow details: %#v", record.details)
	}

	close(releaseFirst)
	if got := receiveString(t, sent, "second send"); got != "e3" {
		t.Fatalf("drop-oldest should keep e3, got %s", got)
	}
	if got := receiveString(t, sent, "third send"); got != "e4" {
		t.Fatalf("drop-oldest should keep e4, got %s", got)
	}
	select {
	case got := <-sent:
		t.Fatalf("dropped event was delivered: %s", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTelegramNotifierStopIsIdempotentAndRejectsLateEnqueue(t *testing.T) {
	sent := make(chan string, 2)
	notifier := telegramservice.NewNotifier(10, func(text string) telegramservice.Result {
		sent <- text
		return telegramservice.Result{Success: true}
	}, nil)
	notifier.Backoff = nil

	notifier.Enqueue(telegramservice.Notification{Event: "before_stop", Text: "before_stop"})
	if got := receiveString(t, sent, "before stop send"); got != "before_stop" {
		t.Fatalf("unexpected first send: %s", got)
	}

	if err := notifier.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := notifier.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !notifier.Stopped() {
		t.Fatal("notifier should be stopped")
	}

	notifier.Enqueue(telegramservice.Notification{Event: "after_stop", Text: "after_stop"})
	select {
	case got := <-sent:
		t.Fatalf("late enqueue delivered after stop: %s", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestTelegramNotifierStopBeforeStartPreventsStart(t *testing.T) {
	notifier := telegramservice.NewNotifier(10, func(string) telegramservice.Result {
		t.Fatal("stopped notifier delivered event")
		return telegramservice.Result{Success: true}
	}, nil)

	if err := notifier.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	notifier.Enqueue(telegramservice.Notification{Event: "after_stop", Text: "after_stop"})

	if !notifier.Stopped() || notifier.Started() || notifier.QueueLen() != 0 {
		t.Fatalf("unexpected notifier state after stop-before-start: started=%v stopped=%v queue=%d", notifier.Started(), notifier.Stopped(), notifier.QueueLen())
	}
}

func stopNotifierAfterTest(t *testing.T, notifier *telegramservice.Notifier) {
	t.Helper()
	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = notifier.Stop(stopCtx)
	})
}

func receiveString(t *testing.T, ch <-chan string, label string) string {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for %s", label)
		return ""
	}
}

func waitForTestChannel(t *testing.T, ch <-chan struct{}, timeout time.Duration, label string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(label)
	}
}
