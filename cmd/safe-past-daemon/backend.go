package main

import (
	"fmt"
	"runtime"

	"golang.design/x/hotkey"

	"github.com/idarciooliveira/safe-past/internal/hotkeymgr"
)

// libReg adapts golang.design/x/hotkey to the manager interface.
// The pump exits on Close; the lib channel is never ranged directly so a
// swapped-out hotkey leaks no goroutine.
type libReg struct {
	hk   *hotkey.Hotkey
	key  <-chan hotkey.Event
	done chan struct{}
	out  chan struct{}
}

func newLibReg(hk *hotkey.Hotkey) *libReg {
	r := &libReg{hk: hk, key: hk.Keydown(), done: make(chan struct{}), out: make(chan struct{}, 1)}
	go r.pump()
	return r
}

func (r *libReg) pump() {
	for {
		select {
		case <-r.done:
			return
		case _, ok := <-r.key:
			if !ok {
				return
			}
			select {
			case r.out <- struct{}{}:
			case <-r.done:
				return
			}
		}
	}
}

func (r *libReg) Keydown() <-chan struct{} { return r.out }

func (r *libReg) Close() error {
	close(r.done)
	return r.hk.Unregister()
}

// libBackend registers a real OS global hotkey for canonical mods/key.
func libBackend(mods []string, key string) (hotkeymgr.Registration, error) {
	lm, err := mapMods(mods)
	if err != nil {
		return nil, err
	}
	lk, err := mapKey(key)
	if err != nil {
		return nil, err
	}
	hk := hotkey.New(lm, lk)
	if err := hk.Register(); err != nil {
		return nil, fmt.Errorf("%w (%s)", err, registerHint())
	}
	return newLibReg(hk), nil
}

func registerHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "grant Accessibility: System Settings → Privacy & Security → Accessibility"
	case "linux":
		return "needs X11 ($DISPLAY); on Wayland use tray 'Scrub now' instead"
	case "windows":
		return "another app may already own this hotkey"
	default:
		return "registration failed"
	}
}
