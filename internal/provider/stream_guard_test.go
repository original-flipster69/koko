package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStallGuardFiresWhenIdle(t *testing.T) {
	ctx, guard := newStallGuard(context.Background(), 20*time.Millisecond)
	defer guard.done()
	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), errStreamStalled) {
			t.Errorf("cause = %v, want errStreamStalled", context.Cause(ctx))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("guard did not fire on idle")
	}
}

func TestStallGuardProgressKeepsAlive(t *testing.T) {
	ctx, guard := newStallGuard(context.Background(), 60*time.Millisecond)
	defer guard.done()
	for i := 0; i < 6; i++ {
		time.Sleep(20 * time.Millisecond)
		guard.progress()
	}
	select {
	case <-ctx.Done():
		t.Fatal("guard fired despite steady progress")
	default:
	}
}

func TestStallGuardParentCancelPropagates(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	ctx, guard := newStallGuard(parent, time.Hour)
	defer guard.done()
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("child ctx not canceled when parent canceled")
	}
}

func TestStreamErrClassifies(t *testing.T) {
	stalled, guard := newStallGuard(context.Background(), time.Millisecond)
	defer guard.done()
	<-stalled.Done()
	if got := streamErr(stalled, errors.New("read failed")); !errors.Is(got, errStreamStalled) {
		t.Errorf("stalled stream should surface errStreamStalled, got %v", got)
	}

	parent, cancel := context.WithCancel(context.Background())
	canceled, guard2 := newStallGuard(parent, time.Hour)
	defer guard2.done()
	cancel()
	<-canceled.Done()
	scanErr := errors.New("connection closed")
	if got := streamErr(canceled, scanErr); !errors.Is(got, scanErr) {
		t.Errorf("user-canceled stream should surface the scan error, got %v", got)
	}
}
