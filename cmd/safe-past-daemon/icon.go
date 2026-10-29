package main

import (
	"encoding/base64"
	"log"
)

// iconData is a 64x64 teal rounded square with a white check (tray icon).
// Regenerate with a designer asset before the 1.0 packaging pass.
var iconData = mustDecodeIcon(
	"iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAYAAACqaXHeAAABCElEQVR42u3byw0CMQxF0TRA3xRAv7BDSDCZIbGdF/taYp97FjAf3Nrg3B73p9KneY5abBjKruEmEFnihxCyxf+FkDX+EkL2+C5ClfhDhNIA1eK/EAAAAIC6AFXj3wgAAADAdof+nHIAv6YMQG/SA5xNaoArkxbAM14ewDseAGWAiHhZgKh4SYDIeDmA6HhzgJmDrog3BZg58Kp4M4DZg6cGODv8yvgwgKOI1fHu3wG9GIV411+BXpRKvMt1gMVs/Uxwp3i3K8HyAKMI6R6Lq8eH3Awpx4fdDarGh94OK8aHPw9Qi+flKAAAAMCfpAAAAAAA2BcAgJUZlqZYm2NxktVZlqcrrs+/AHnNwh9Yk/7UAAAAAElFTkSuQmCC",
)

func mustDecodeIcon(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Fatalf("bad tray icon: %v", err)
	}
	return b
}
