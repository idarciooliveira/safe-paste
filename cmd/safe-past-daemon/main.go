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
	state.mgr = mgr
	if err := mgr.SetHotkey(cfg.Hotkey); err != nil {
		// Headless has no fallback UI: fail fast. Tray mode keeps
		// running so "Scrub clipboard now" stays reachable (notably
		// on pure Wayland where X11 registration always fails).
		if *noTray {
			log.Fatalf("safe-past-daemon: hotkey %q: %v", cfg.Hotkey, err)
		}
		log.Printf("safe-past-daemon: hotkey %q: %v (continuing without hotkey; use tray 'Scrub now')", cfg.Hotkey, err)
		setStatus("hotkey unavailable (" + cfg.Hotkey + "): " + err.Error() + " — use 'Scrub now'")
	}

	go state.watchConfig()

	if *noTray {
		state.runHeadless()
		return
	}
	runTray(state)
}

// daemon holds mutable runtime state behind a mutex: the tray and the
// config watcher read cfg while onHotkey scrubs with it.
// scrubMu serializes clipboard read→scrub→write so overlapping hotkey
// and tray scrubs cannot interleave and let an older read win.
type daemon struct {
	verbose bool
	mu      sync.RWMutex
	cfg     config.Config
	mgr     *hotkeymgr.Manager
	scrubMu sync.Mutex
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
		// Still enabled but the binary may have moved (update installs
		// to a new folder) or the preserved --config path may differ:
		// rewrite a stale entry instead of launching the old path.
		if want {
			if cur, cerr := autostart.IsCurrent(); cerr != nil {
				if d.verbose {
					log.Printf("autostart check current: %v", cerr)
				}
			} else if !cur {
				if serr := autostart.SetEnabled(true); serr != nil {
					log.Printf("safe-past-daemon: autostart refresh: %v", serr)
					return
				}
				if d.verbose {
					log.Printf("autostart refreshed to current executable")
				}
			}
		}
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
// Serialized by scrubMu: every hotkey and tray scrub funnels through here,
// so a slow scrub cannot read stale text and overwrite a newer result.
func (d *daemon) scrubOnce() int {
	d.scrubMu.Lock()
	defer d.scrubMu.Unlock()
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
		next := cfg
		if cfg.Hotkey != old.Hotkey {
			if d.mgr == nil {
				log.Printf("safe-past-daemon: hotkey %q: no manager", cfg.Hotkey)
				next.Hotkey = old.Hotkey
			} else if err := d.mgr.SetHotkey(cfg.Hotkey); err != nil {
				// Keep the last working hotkey: the old shortcut is
				// still registered, and d.cfg must reflect reality so
				// freeing the key and saving again can retry.
				log.Printf("safe-past-daemon: hotkey %q: %v (keeping %q)", cfg.Hotkey, err, old.Hotkey)
				setStatus("hotkey " + cfg.Hotkey + " unavailable, keeping " + displayHotkey(old.Hotkey))
				next.Hotkey = old.Hotkey
			} else {
				setStatus("Hotkey: " + displayHotkey(cfg.Hotkey))
			}
		}
		d.mu.Lock()
		d.cfg = next
		d.mu.Unlock()
		if d.verbose {
			log.Printf("config reloaded: hotkey=%s mode=%s", next.Hotkey, next.Mode)
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
