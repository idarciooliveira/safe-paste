// Package hotkeymgr owns one live global hotkey with hot-swap.
//
// It validates specs with the pure internal/hotkey parser and registers
// through an injectable Backend: the real OS hotkey in the daemon, a fake
// in tests. Headless-safe: no display, tray, or cgo here.
package hotkeymgr

import (
	"fmt"
	"sync"

	ihotkey "github.com/idarciooliveira/safe-past/internal/hotkey"
)

// Registration is one live global hotkey.
type Registration interface {
	Keydown() <-chan struct{}
	Close() error
}

// Backend builds a registration for canonical mods/key.
type Backend func(mods []string, key string) (Registration, error)

// Manager keeps exactly one hotkey registered; SetHotkey swaps it.
type Manager struct {
	mu      sync.Mutex
	backend Backend
	onScrub func()
	spec    string
	cur     Registration
	stop    chan struct{}
}

// New builds a manager around the real OS backend (provided by the caller
// platform code via NewWithBackend).
func New(onScrub func(), b Backend) *Manager {
	return &Manager{backend: b, onScrub: onScrub}
}

// SetHotkey validates spec ("Ctrl+Shift+V"), registers it, and swaps out
// the previous hotkey. The old loop stops before the new one starts.
func (m *Manager) SetHotkey(spec string) error {
	h, err := ihotkey.Parse(spec)
	if err != nil {
		return err
	}
	reg, err := m.backend(h.Mods, h.Key)
	if err != nil {
		return fmt.Errorf("register %s: %w", h.String(), err)
	}
	m.mu.Lock()
	if m.stop != nil {
		close(m.stop)
	}
	if m.cur != nil {
		_ = m.cur.Close()
	}
	stop := make(chan struct{})
	m.cur = reg
	m.stop = stop
	m.spec = h.String()
	m.mu.Unlock()
	go m.loop(reg, stop)
	return nil
}

func (m *Manager) loop(reg Registration, stop chan struct{}) {
	ch := reg.Keydown()
	for {
		select {
		case <-stop:
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			m.onScrub()
		}
	}
}

// Spec returns the canonical hotkey currently registered ("" if none).
func (m *Manager) Spec() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.spec
}

// Close unregisters the current hotkey and stops its loop.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	if m.cur != nil {
		_ = m.cur.Close()
		m.cur = nil
	}
}
