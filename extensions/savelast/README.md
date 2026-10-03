# savelast

A Go PiG extension that saves the latest assistant response on the active
session branch.

## Usage

```text
/savelast
/savelast path/to/output.md
```

The extension writes only text content from the latest assistant message. It
skips thinking, tool-call, image, and malformed blocks. A blank latest response
is not replaced by an older response. Relative paths resolve against the
command context's current working directory, parent directories are created,
and existing files are overwritten without confirmation.

The extension follows PiG's active-branch decision for this port. The original
`atomdmac/pi-savelast` source scans the complete session entry list; PiG uses
`SessionManager().GetBranch(nil)` so abandoned branches are not selected.
