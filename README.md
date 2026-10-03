# tty

Terminal building blocks for Wippy, in pure Go with no cgo:

- `text`: display width over grapheme clusters, cut/truncate/strip, wrap, SGR styles, boxes and layout.
- `canvas`: styled cell buffers rendered to minimal ANSI rows.
- `input`: decoding of terminal input (keys incl. kitty protocol, mouse, paste, focus, size, Windows console).
- `vt`: VT/xterm terminal emulator for PTY-backed windows.
