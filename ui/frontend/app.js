// safe-past Wails frontend. Talks to window.go.backend.App when running
// inside Wails; with ?demo=1 it renders a static sample (docs/screenshots).
(function () {
  "use strict";
  const $ = (id) => document.getElementById(id);
  const input = $("input"), output = $("output"), stats = $("stats");
  const modeSel = $("mode"), toggles = $("toggles"), entCount = $("entCount");
  const copyBtn = $("copyBtn"), pasteBtn = $("pasteBtn"), clearBtn = $("clearBtn");
  const inCount = $("inCount");
  const params = new URLSearchParams(location.search);
  const bridge = window.go && window.go.backend && window.go.backend.App;

  const TOK = /(\[[A-Z][A-Z0-9_]*_\d+\]|<[A-Z][A-Z0-9_]*>|[a-f0-9]{16})/g;
  function highlight(text) {
    output.textContent = "";
    const parts = text.split(TOK);
    for (const p of parts) {
      if (TOK.test(p)) {
        TOK.lastIndex = 0;
        const s = document.createElement("span");
        s.className = "tok";
        s.textContent = p;
        output.appendChild(s);
      } else {
        output.appendChild(document.createTextNode(p));
      }
    }
  }

  let timer = 0;
  async function run() {
    const text = input.value;
    inCount.textContent = text ? text.length + " chars" : "";
    copyBtn.disabled = !text;
    if (!text) { output.textContent = ""; stats.textContent = "De-identified output appears here."; return; }
    const disabled = [...toggles.querySelectorAll("input:not(:checked)")].map((c) => c.value);
    if (!bridge) return; // demo mode fills output statically below
    try {
      const res = await bridge.Redact({ text, mode: modeSel.value, disabled, maskKeep: 4, maskChar: "*", extra: [] });
      highlight(res.text);
      stats.textContent = res.stats;
    } catch (e) {
      stats.textContent = "Redaction failed: " + e;
    }
  }
  function schedule() { clearTimeout(timer); timer = setTimeout(run, 180); }

  async function boot() {
    if (params.has("demo")) {
      input.value = "ERROR db connect failed host=192.168.1.10 user=bob@example.com\nphone +1-555-123-4567 password=hunter2\nkey=AKIAIOSFODNN7EXAMPLE";
      highlight("ERROR db connect failed host=[IP_1] user=[EMAIL_1]\nphone [PHONE_1] password=[SECRET_1]\nkey=[AWS_KEY_1]");
      stats.textContent = "scrubbed 5 value(s): AWS_KEY=1 EMAIL=1 IP=1 PHONE=1 SECRET=1";
      inCount.textContent = input.value.length + " chars";
      copyBtn.disabled = false;
    }
    let rules = [];
    if (bridge) {
      try { rules = await bridge.ListRules(); } catch (e) { /* offline */ }
    }
    if (!rules.length) {
      rules = ["IP", "EMAIL", "PHONE", "SECRET", "AWS_KEY", "JWT", "IBAN", "BI_AO"].map((p) => ({ name: p, placeholder: p }));
    }
    const seen = [...new Set(rules.map((r) => r.placeholder))].sort();
    entCount.textContent = "(" + seen.length + ")";
    for (const p of seen) {
      const lab = document.createElement("label");
      const cb = document.createElement("input");
      cb.type = "checkbox"; cb.checked = true; cb.value = p;
      cb.addEventListener("change", schedule);
      lab.appendChild(cb);
      lab.appendChild(document.createTextNode(p));
      toggles.appendChild(lab);
    }
    if (bridge && input.value) run();
  }

  input.addEventListener("input", schedule);
  modeSel.addEventListener("change", run);
  clearBtn.addEventListener("click", () => { input.value = ""; run(); input.focus(); });
  pasteBtn.addEventListener("click", async () => {
    if (bridge) {
      try { input.value = await bridge.ReadClipboard(); run(); return; } catch (e) { /* fall through */ }
    }
    try { input.value = await navigator.clipboard.readText(); run(); }
    catch (e) { stats.textContent = "Clipboard read blocked — paste with Ctrl+V."; }
  });
  copyBtn.addEventListener("click", async () => {
    const text = output.innerText;
    if (!text) return;
    if (bridge) { try { await bridge.WriteClipboard(text); } catch (e) { /* fall through */ } }
    try { await navigator.clipboard.writeText(text); stats.textContent += "  ·  copied ✓"; }
    catch (e) { stats.textContent = "Copy blocked — select the output and copy manually."; }
  });

  boot();
})();
