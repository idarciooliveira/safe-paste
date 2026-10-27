package hotkeymgr

import (
	"sync/atomic"
	"testing"
	"time"
)

type fakeReg struct {
	ch     chan struct{}
	closed atomic.Bool
}

func (f *fakeReg) Keydown() <-chan struct{} { return f.ch }
func (f *fakeReg) Close() error {
	f.closed.Store(true)
	return nil
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func TestSetAndFire(t *testing.T) {
	reg := &fakeReg{ch: make(chan struct{}, 1)}
	var scrubs atomic.Int32
	m := New(func() { scrubs.Add(1) }, func(mods []string, key string) (Registration, error) {
		if key != "V" {
			t.Fatalf("key = %q", key)
		}
		return reg, nil
	})
	if err := m.SetHotkey("Ctrl+Shift+V"); err != nil {
		t.Fatal(err)
	}
	if m.Spec() != "Ctrl+Shift+V" {
		t.Fatalf("spec = %q", m.Spec())
	}
	reg.ch <- struct{}{}
	waitFor(t, "scrub", func() bool { return scrubs.Load() == 1 })
	m.Close()
}

func TestSwapClosesOld(t *testing.T) {
	first := &fakeReg{ch: make(chan struct{}, 1)}
	second := &fakeReg{ch: make(chan struct{}, 1)}
	n := 0
	var scrubs atomic.Int32
	m := New(func() { scrubs.Add(1) }, func(mods []string, key string) (Registration, error) {
		n++
		if n == 1 {
			return first, nil
		}
		return second, nil
	})
	if err := m.SetHotkey("Ctrl+Shift+V"); err != nil {
		t.Fatal(err)
	}
	if err := m.SetHotkey("Ctrl+Shift+X"); err != nil {
		t.Fatal(err)
	}
	if !first.closed.Load() {
		t.Fatal("old registration not closed")
	}
	if m.Spec() != "Ctrl+Shift+X" {
		t.Fatalf("spec = %q", m.Spec())
	}
	// Old loop exits via its closed stop channel (its event channel is empty,
	// so stop is the only ready case). Afterwards nothing drains first.ch,
	// so firing the old hotkey is a no-op by construction.
	time.Sleep(50 * time.Millisecond)
	first.ch <- struct{}{}
	time.Sleep(50 * time.Millisecond)
	if scrubs.Load() != 0 {
		t.Fatal("old loop still scrubbing")
	}
	second.ch <- struct{}{}
	waitFor(t, "scrub", func() bool { return scrubs.Load() == 1 })
	m.Close()
}

func TestRejectsBadSpecWithoutBackend(t *testing.T) {
	called := false
	m := New(func() {}, func(mods []string, key string) (Registration, error) {
		called = true
		return &fakeReg{ch: make(chan struct{})}, nil
	})
	if err := m.SetHotkey("V"); err == nil {
		t.Fatal("expected error")
	}
	if called {
		t.Fatal("backend must not run for invalid spec")
	}
	if m.Spec() != "" {
		t.Fatalf("spec = %q", m.Spec())
	}
}

func TestSameCanonicalSpecSkipsBackend(t *testing.T) {
	calls := 0
	m := New(func() {}, func(mods []string, key string) (Registration, error) {
		calls++
		return &fakeReg{ch: make(chan struct{}, 1)}, nil
	})
	if err := m.SetHotkey("Ctrl+Shift+V"); err != nil {
		t.Fatal(err)
	}
	// Same canonical spec in different spelling: no re-register, or the
	// OS rejects an already-owned hotkey and logs a bogus "unavailable".
	if err := m.SetHotkey("ctrl+shift+v"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("backend calls = %d, want 1", calls)
	}
	m.Close()
}
