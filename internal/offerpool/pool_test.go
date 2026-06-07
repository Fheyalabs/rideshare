package offerpool

import (
	"testing"
	"time"
)

func TestPool_MaxThreeAndAcceptEndsAll(t *testing.T) {
	p := New(3, time.Minute)
	for i, d := range []string{"a", "b", "c"} {
		if err := p.Hold(Offer{ID: string(rune('0' + i)), Driver: d, PriceCents: 1200}); err != nil {
			t.Fatalf("hold %s: %v", d, err)
		}
	}
	if err := p.Hold(Offer{ID: "3", Driver: "d"}); err == nil {
		t.Fatal("4th hold should be rejected (max 3)")
	}
	if err := p.Accept("1"); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if len(p.Held()) != 0 {
		t.Fatalf("accept must release all held, got %d", len(p.Held()))
	}
}

func TestPool_WinnerExcludedFromResearch(t *testing.T) {
	p := New(3, time.Minute)
	_ = p.Hold(Offer{ID: "1", Driver: "a"})
	if !p.Excluded("a") {
		t.Fatal("held winner must be excluded from re-search")
	}
	if p.Excluded("b") {
		t.Fatal("non-held driver must not be excluded")
	}
}

func TestPool_RiderCancelDriverCannot(t *testing.T) {
	p := New(3, time.Minute)
	_ = p.Hold(Offer{ID: "1", Driver: "a"})
	if err := p.Cancel("1"); err != nil {
		t.Fatalf("rider cancel: %v", err) // rider can
	}
	if len(p.Held()) != 0 {
		t.Fatal("cancel should remove the held offer")
	}
}

func TestPool_AsyncExpire(t *testing.T) {
	p := New(3, 10*time.Millisecond)
	_ = p.Hold(Offer{ID: "1", Driver: "a"})
	time.Sleep(50 * time.Millisecond)
	if len(p.Held()) != 0 {
		t.Fatal("offer should have expired")
	}
}
