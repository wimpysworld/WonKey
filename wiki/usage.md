# Usage and configuration

[Wiki index](README.md) · [Project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md)

Plan settings offline before any hardware test. Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run all commands from the project root, not from `wiki/`, so that relative fixture paths work.

## Plan against the original capture

These commands are offline. They do not change settings or open a device.

```sh
original=/home/martin/.local/state/agent-reviews/key/worktree-key/run-20260911T150408Z-dMkT7Y/captures/20260911T162302.726391430Z-3220202532
./wonkey plan --capture "$original" --rgb-mode 1 --red 0 --green 0 --blue 255
./wonkey plan --capture "$original" --key f13
```

The labelled fixture `internal/xfkey/testdata/hardware-20260911` is also a valid `--capture` directory.
Planning checks the completion marker, identity, raw replies, and reconstructed configuration for consistency.
Output includes current/intended configurations, changed offsets, and upload/commit payloads. A no-op has no upload payloads.
A saved plan never authorises a later write. Apply always reads and backs up current device state again.

Only explicitly supplied fields change. All other bytes remain equal to the current configuration.
Supported current layouts are single-key Enter or F13, with a known trigger, modifier mask, and RGB mode. Unknown layouts or modes fail closed, even for RGB-only changes.
There is no conversion of macros, mouse/media commands, multi-key layouts, or unknown keys.

| Flag | Values | Configuration byte |
|---|---|---:|
| `--key` | `enter` (usage `28`), `f13` (usage `68`) | 4 |
| `--trigger` | 1 press, 2 release, 3 both | 1 |
| `--modifiers` | 0–15, left Ctrl=1, Shift=2, Alt=4, GUI=8 | 2 |
| `--rgb-mode` | API index 0–7, stored as index+1 | 124 |
| `--red`, `--green`, `--blue` | 0–255 each | 125–127 |

Bytes 0 and 3 must be `00` and `01`. The tool preserves those bytes and bytes 5–123.
There are no default key or RGB changes. At least one explicit field is required.

## RGB modes

RGB vendor labels, not verified effects:

| API index | Stored byte | Vendor interpretation |
|---:|---:|---|
| 0 | 01 | Full-colour gradient |
| 1 | 02 | Single-colour steady |
| 2 | 03 | Single-colour flowing |
| 3 | 04 | Flash on click |
| 4 | 05 | Neon flowing |
| 5 | 06 | Lights off |
| 6 | 07 | On while pressed, off on release |
| 7 | 08 | Toggle on click |

No brightness or speed field is established.

For device access, permissions, backup, and restore, see [hardware operations](hardware.md).
For offline parsing and full replacement previews, see the [protocol reference](protocol.md#existing-offline-operations).
