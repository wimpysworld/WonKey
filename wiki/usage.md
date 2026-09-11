# Usage and configuration

[Wiki index](README.md) · [Project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md)

Plan settings offline before any hardware test. Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run all commands from the project root, not from `wiki/`, so that relative fixture paths work.

## Read and plan settings

`wonkey show --device 1-1.2` queries identity and all three settings replies in memory. It validates the replies and creates no files.

Plan against a durable apply backup:

These commands are offline. They do not change settings or open a device.

```sh
./wonkey plan --capture "$capture" --lighting steady --colour 0000ff
./wonkey plan --capture "$capture" --key f13
```

The labelled hardware fixture has no authentic `result.json` completion marker, so WonKey does not accept it as a complete backup.
Planning checks the completion marker, identity, raw replies, and reconstructed configuration for consistency.
Human output lists semantic changes. Use `--json` for script output.
A saved plan never authorises a later write. Apply always reads and backs up current device state again.

Only explicitly supplied fields change. All other bytes remain equal to the current configuration.
Supported current layouts are single-key Enter or F13, with a known trigger, modifier mask, and RGB mode. Unknown layouts or modes fail closed, even for RGB-only changes.
There is no conversion of macros, mouse/media commands, multi-key layouts, or unknown keys.

| Flag | Values | Configuration byte |
|---|---|---:|
| `--key` | `enter` (usage `28`), `f13` (usage `68`) | 4 |
| `--trigger` | `press`, `release`, `both` | 1 |
| `--modifiers` | `none`, or comma-separated `ctrl,shift,alt,gui` | 2 |
| `--lighting` | `gradient`, `steady`, `flowing`, `flash`, `neon`, `off`, `held`, `toggle` | 124 |
| `--colour` | `RRGGBB` or `#RRGGBB` | 125–127 |

Bytes 0 and 3 must be `00` and `01`. The tool preserves those bytes and bytes 5–123.
There are no default key or lighting changes. At least one explicit field is required.

## Lighting

These lighting names are vendor labels, not verified effects:

| Compatibility index | Stored byte | Vendor interpretation |
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

The expert `protocol preview` command uses numeric protocol values for all fields.

`plan` and `apply` keep the old numeric settings for existing scripts. They accept `--trigger` values `1` to `3`, `--modifiers` values `0` to `15`, `--rgb-mode` values `0` to `7`, and separate `--red`, `--green`, and `--blue` values `0` to `255`. The numeric-only flags stay hidden from the primary help. Do not combine numeric compatibility values with friendly `--trigger`, `--modifiers`, `--lighting`, or `--colour` values in one command.

For device access, permissions, backup, and restore, see [hardware operations](hardware.md).
For offline parsing and full replacement previews, see the [protocol reference](protocol.md#existing-offline-operations).
