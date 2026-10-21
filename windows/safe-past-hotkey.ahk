; safe-past Windows global hotkey (AutoHotkey v2)
; Install AutoHotkey v2, then double-click this file.
; Ctrl+Shift+V anywhere: cleans clipboard in place + tooltip confirmation.
; Adjust EXE path to where you put safe-past.exe.

#Requires AutoHotkey v2.0
^+v:: {
    exe := "C:\Tools\safe-past\safe-past.exe"
    try {
        RunWait('"' exe '" --clipboard --copy --stats', , "Hide")
        ToolTip "safe-past: clipboard scrubbed ✓"
        SetTimer () => ToolTip(), -1500
    } catch as e {
        MsgBox "safe-past failed: " e.Message
    }
}
