; prompt-parse Windows global hotkey (AutoHotkey v2)
; Install AutoHotkey v2, then double-click this file.
; Ctrl+Shift+V anywhere: cleans clipboard in place + tooltip confirmation.
; Adjust EXE path to where you put prompt-parse.exe.

#Requires AutoHotkey v2.0
^+v:: {
    exe := "C:\Tools\prompt-parse\prompt-parse.exe"
    try {
        RunWait('"' exe '" --clipboard --copy --stats', , "Hide")
        ToolTip "prompt-parse: clipboard scrubbed ✓"
        SetTimer () => ToolTip(), -1500
    } catch as e {
        MsgBox "prompt-parse failed: " e.Message
    }
}
