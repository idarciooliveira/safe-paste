package main

import (
	"log"
	"sync"

	"fyne.io/systray"

	"github.com/idarciooliveira/safe-past/internal/autostart"
	"github.com/idarciooliveira/safe-past/internal/config"
)

// Tray menu. All mutations funnel through refreshMenu so the watcher,
// the hotkey loop, and clicks never disagree about what is shown.
var (
	trayState *daemon

	trayMu     sync.Mutex
	trayUp     bool
	lastStatus = "ready — copy logs, press the hotkey, paste clean"

	mStatus    *systray.MenuItem
	mScrub     *systray.MenuItem
	mAutostart *systray.MenuItem
	mConfig    *systray.MenuItem
	mQuit      *systray.MenuItem
)

// setStatus updates the tooltip + status line; safe before tray starts
// (headless / --scrub-now) and from any goroutine.
func setStatus(s string) {
	trayMu.Lock()
	lastStatus = s
	up := trayUp
	trayMu.Unlock()
	if !up {
		return
	}
	mStatus.SetTitle("safe-past · " + s)
	systray.SetTooltip("safe-past · " + s)
}

// refreshMenu re-renders titles/checkbox from current config + status.
func refreshMenu() {
	trayMu.Lock()
	up := trayUp
	status := lastStatus
	trayMu.Unlock()
	if !up || trayState == nil {
		return
	}
	cfg := trayState.getConfig()
	mStatus.SetTitle("safe-past · " + status)
	mStatus.SetTooltip("Hotkey: " + displayHotkey(cfg.Hotkey))
	systray.SetTooltip("safe-past (" + displayHotkey(cfg.Hotkey) + ") · " + status)
	if cfg.Autostart {
		mAutostart.Check()
	} else {
		mAutostart.Uncheck()
	}
}

func runTray(d *daemon) {
	trayState = d
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("safe-past")

	mStatus = systray.AddMenuItem("safe-past · starting…", "current hotkey and last result")
	mStatus.Disable()
	systray.AddSeparator()
	mScrub = systray.AddMenuItem("Scrub clipboard now", "scrub the clipboard in place right away")
	mAutostart = systray.AddMenuItemCheckbox("Start at login", "launch the daemon on login",
		trayState.getConfig().Autostart)
	mConfig = systray.AddMenuItem("Edit config (hotkey…)", "open config.json in your editor")
	systray.AddSeparator()
	mQuit = systray.AddMenuItem("Quit", "unregister hotkey and quit")

	trayMu.Lock()
	trayUp = true
	trayMu.Unlock()

	// If startup hotkey registration failed, main() already stored the
	// error in lastStatus: keep it instead of claiming the hotkey works.
	if trayState.mgr == nil || trayState.mgr.Spec() == "" {
		refreshMenu()
	} else {
		setStatus("hotkey " + displayHotkey(trayState.getConfig().Hotkey))
		refreshMenu()
	}
	go watchMenuClicks()
}

func watchMenuClicks() {
	for {
		select {
		case <-mScrub.ClickedCh:
			go trayState.onHotkey()
		case <-mAutostart.ClickedCh:
			toggleAutostart()
		case <-mConfig.ClickedCh:
			if err := openPath(configPathOf()); err != nil {
				setStatus("cannot open config: " + err.Error())
			}
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

// toggleAutostart flips the setting, persists it (the file watcher then
// converges), and applies it immediately for snappy UI.
// It reloads the latest file first: saving the stale in-memory copy would
// overwrite a hotkey/mode edit made within the watcher poll interval.
func toggleAutostart() {
	latest, err := config.Load()
	if err != nil {
		// File unreadable: fall back to flipping the in-memory copy.
		latest = trayState.getConfig()
		latest.Autostart = !latest.Autostart
	} else {
		latest.Autostart = !latest.Autostart
	}
	if err := latest.Save(); err != nil {
		setStatus("config save failed: " + err.Error())
		return
	}
	trayState.mu.Lock()
	trayState.cfg = latest
	trayState.mu.Unlock()
	trayState.syncAutostart()
	refreshMenu()
	if on, err := autostart.Enabled(); err == nil && trayState.verbose {
		log.Printf("autostart -> %v", on)
	}
}

func onExit() {
	if trayState != nil && trayState.mgr != nil {
		trayState.mgr.Close()
	}
}
