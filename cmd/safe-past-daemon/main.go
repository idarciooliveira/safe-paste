// Command safe-past-daemon runs the tray icon + global hotkey.
//
// Default flow: load config (creating defaults on first run), sync the OS
// login item, register the hotkey, show the tray. Pressing the hotkey
// scrubs the clipboard in place via the redact engine.
//
// Flags:
//
//	--config PATH  use a custom config file (also SAFE_PAST_CONFIG_FILE)
//	--no-tray      headless: hotkey only, no tray icon (Wayland/servers)
//	--scrub-now    scrub the clipboard once and exit (no tray, no hotkey)
//	--verbose      log each scrub + config reload to stderr
//	--version      print version and exit
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/atotto/clipboard"

	"github.com/idarciooliveira/safe-past/internal/autostart"
	"github.com/idarciooliveira/safe-past/internal/config"
	"github.com/idarciooliveira/safe-past/internal/hotkey"
	"github.com/idarciooliveira/safe-past/internal/hotkeymgr"
	"github.com/idarciooliveira/safe-past/internal/scrub"
)

var version = "dev"

func main() {
	var (
		configPath = flag.String("config", "", "config file path (default: OS user config dir)")
		noTray     = flag.Bool("no-tray", false, "headless: register hotkey without tray icon")
		scrubNow   = flag.Bool("scrub-now", false, "scrub clipboard once and exit")
		verbose    = flag.Bool("verbose", false, "log scrubs and config reloads")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *configPath != "" {
		_ = os.Setenv(config.EnvFile, *configPath)
	}
	if *showVer {
		fmt.Println("safe-past-daemon " + version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("safe-past-daemon: %v", err)
	}
	// First run: persist defaults so the GUI has a file to edit.
	if _, err := os.Stat(configPathOf()); os.IsNotExist(err) {
		if err := cfg.Save(); err != nil && *verbose {
			log.Printf("save default config: %v", err)
		}
	}
	// Normalize the stored hotkey spelling (Cmd -> Super etc.).
	if norm, err := hotkey.Normalize(cfg.Hotkey); err == nil && norm != cfg.Hotkey {
		cfg.Hotkey = norm
		_ = cfg.Save()
	}

	state := &daemon{verbose: *verbose, cfg: cfg}

	if *scrubNow {
		os.Exit(state.scrubOnce())
	}

	state.syncAutostart()

	mgr := hotkeymgr.New(func() { state.onHotkey() }, libBackend)
	if err := mgr.SetHotkey(cfg.Hotkey); err != nil {
		log.Fatalf("safe-past-daemon: hotkey %q: %v", cfg.Hotkey, err)
	}
	state.mgr = mgr

	go state.watchConfig()

	if *noTray {
		state.runHeadless()
		return
	}
	runTray(state)
}

// daemon holds mutable runtime state behind a mutex: the tray and the
// config watcher read cfg while onHotkey scrubs with it.
type daemon struct {
	verbose bool
	mu      sync.RWMutex
	cfg     config.Config
	mgr     *hotkeymgr.Manager
}

func configPathOf() string {
	p, err := config.Path()
	if err != nil {
		return ""
	}
	return p
}

func (d *daemon) getConfig() config.Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

// syncAutostart aligns the OS login item with cfg.Autostart.
func (d *daemon) syncAutostart() {
	on, err := autostart.Enabled()
	if err != nil && d.verbose {
		log.Printf("autostart check: %v", err)
	}
	want := d.getConfig().Autostart
	if on == want {
		return
	}
	if err := autostart.SetEnabled(want); err != nil {
		log.Printf("safe-past-daemon: autostart: %v", err)
		return
	}
	if d.verbose {
		log.Printf("autostart -> %v", want)
	}
}

// onHotkey scrubs the clipboard; called from the hotkey loop and the tray.
func (d *daemon) onHotkey() {
	if code := d.scrubOnce(); d.verbose {
		log.Printf("hotkey: exit %d", code)
	}
}

// scrubOnce reads, scrubs, and writes back the clipboard.
// Returns a process exit code (0 ok, 1 retryable/empty, 2 config error).
func (d *daemon) scrubOnce() int {
	cfg := d.getConfig()
	text, err := clipboard.ReadAll()
	if err != nil {
		log.Printf("safe-past-daemon: clipboard read: %v", readHint(err))
		return 1
	}
	res, err := scrub.ScrubText(text, cfg)
	if err != nil {
		log.Printf("safe-past-daemon: scrub: %v", err)
		return 2
	}
	if res.Count == 0 && res.Text == "" {
		if d.verbose {
			log.Print("safe-past-daemon: clipboard empty, nothing to do")
		}
		setStatus("Clipboard empty — copy logs first")
		return 1
	}
	if err := clipboard.WriteAll(res.Text); err != nil {
		log.Printf("safe-past-daemon: clipboard write: %v", err)
		return 1
	}
	setStatus(res.Summary)
	if d.verbose {
		log.Print(res.Summary)
	} else {
		log.Print(res.Summary)
	}
	return 0
}

func readHint(err error) string {
	base := err.Error()
	if runtime.GOOS == "linux" {
		return base + " (Linux needs xclip or xsel on PATH; WSL needs win32yank)"
	}
	return base
}

// watchConfig polls the config file and hot-swaps hotkey + autostart.
func (d *daemon) watchConfig() {
	path := configPathOf()
	if path == "" {
		return
	}
	last := mtime(path)
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for range t.C {
		m := mtime(path)
		if m == last {
			continue
		}
		last = m
		cfg, err := config.Load()
		if err != nil {
			log.Printf("safe-past-daemon: reload: %v", err)
			continue
		}
		old := d.getConfig()
		d.mu.Lock()
		d.cfg = cfg
		d.mu.Unlock()
		if d.verbose {
			log.Printf("config reloaded: hotkey=%s mode=%s", cfg.Hotkey, cfg.Mode)
		}
		if cfg.Hotkey != old.Hotkey {
			if err := d.mgr.SetHotkey(cfg.Hotkey); err != nil {
				log.Printf("safe-past-daemon: hotkey %q: %v", cfg.Hotkey, err)
			} else {
				setStatus("Hotkey: " + displayHotkey(cfg.Hotkey))
			}
		}
		if cfg.Autostart != old.Autostart {
			d.syncAutostart()
			refreshMenu()
		}
	}
}

func mtime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

// runHeadless registers the hotkey loop without a tray (servers, Wayland).
func (d *daemon) runHeadless() {
	runtime.LockOSThread() // keep darwin event delivery on the main thread
	if runtime.GOOS == "linux" && os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") == "" {
		log.Print("safe-past-daemon: Wayland detected and no XWayland DISPLAY; " +
			"global hotkey needs X11. Tray 'Scrub now' / desktop shortcut still work.")
	}
	log.Printf("safe-past-daemon %s headless: hotkey %s (Ctrl+C to quit)",
		version, displayHotkey(d.getConfig().Hotkey))
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for s := range sig {
		if s == syscall.SIGHUP {
			continue // watcher picks up the reload
		}
		break
	}
	d.mgr.Close()
}

func displayHotkey(spec string) string {
	h, err := hotkey.Parse(spec)
	if err != nil {
		return spec
	}
	return h.Display(runtime.GOOS == "darwin")
}
