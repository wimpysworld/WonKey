# Usage and configuration

[Wiki index](README.md) · [Project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md)

Use `show` to read settings and `set` to save changes. Offline planning is an advanced option that needs a completed backup.
Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run all commands from the project root, not from `wiki/`, so that relative fixture paths work.

## Read and plan settings

`wonkey show` reads and validates current settings in memory. It creates no files, but it is not offline.
Both commands select the only compatible device. If multiple devices match, use `--device PHYSICAL_PATH`.
Find paths with `wonkey advanced devices`, which reads system metadata.

These `set` examples change stored settings after confirmation:

```sh
./wonkey set key=f13
./wonkey set light=steady:0000ff
./wonkey set key=f13 light=steady:0000ff
./wonkey set --device 1-1.2 key=f13 trigger=release modifiers=ctrl,shift
```

`set` reads identity and settings, shows changes, and asks for the exact answer `write`. A blank answer cancels.
It then creates and validates a durable backup. Changed identity, version, or settings stop the transaction before upload.
`--yes` skips only confirmation, not any safety check. Noninteractive writes require `--yes`.
A no-op or cancellation sends no settings upload or commit and creates no backup.

Preview changes without saving:

```sh
./wonkey set key=f13 light=steady:0000ff --dry-run
```

`--dry-run` reads the device. It creates no files and sends no settings upload or commit.
Use `--capture-root` to set an absolute backup root. Otherwise, WonKey uses non-empty `WONKEY_CAPTURE_ROOT`, then `XFKEY_CAPTURE_ROOT`, then `$HOME/.local/state/xfkey-captures`.

Plan against a completed durable apply backup. `show` does not produce this backup:

These commands are offline. They do not change settings or open a device.

```sh
./wonkey advanced plan "$capture" light=steady:0000ff
./wonkey advanced plan "$capture" key=f13
```

The labelled hardware fixture has no authentic `result.json` completion marker, so WonKey does not accept it as a complete backup.
Planning checks the completion marker, identity, raw replies, and reconstructed configuration for consistency.
Human output lists semantic changes. Use `--json` for script output.
A saved plan never authorises a later write. `set` always reads current device state and backs it up before upload.

Only explicitly supplied fields change. All other bytes remain equal to the current configuration.
Supported current layouts are single-key Enter or F13, with a known trigger, modifier mask, and RGB mode. Unknown layouts or modes fail closed, even for RGB-only changes.
There is no conversion of macros, mouse/media commands, multi-key layouts, or unknown keys.

| Assignment | Values | Configuration byte |
|---|---|---:|
| `key=` | `enter` (usage `28`), `f13` (usage `68`) | 4 |
| `trigger=` | `press`, `release`, `both` | 1 |
| `modifiers=` | `none`, or comma-separated `ctrl,shift,alt,gui` | 2 |
| `lighting=` | `gradient`, `steady`, `flowing`, `flash`, `neon`, `off`, `held`, `toggle` | 124 |
| `colour=` | `RRGGBB` or `#RRGGBB` | 125–127 |
| `light=` | `mode:RRGGBB`, for example `steady:0000ff` | 124–127 |

`key=` preserves trigger and modifiers. `colour=` preserves the mode. `lighting=` preserves the colour.
`light=` explicitly sets both mode and colour. No assignment supplies implicit defaults.
Duplicate or overlapping assignments fail, including `light=steady:0000ff colour=ff0000`. Repeated settings flags also fail.
Do not mix assignments and settings flags in one command.

The old top-level commands remain hidden compatibility aliases. Use `advanced devices`, `advanced plan`, and `advanced protocol` for these tools.
Legacy `apply TARGET key=f13 --write` still requires the target and write gate. It takes a backup before confirmation, including no-op and cancelled transactions.
Use `show --target` to include a copyable target, or use the unchanged JSON `target` field.
The compatibility forms `show PATH`, `plan --capture DIR`, and `apply --target TOKEN` remain accepted.
Do not combine a positional object with its flagged form, even when both values match.
Old `wonkey-target-v1:` tokens remain accepted, and JSON `target` values keep that format.
The settings flags `--key`, `--trigger`, `--modifiers`, `--lighting`, and `--colour` also remain accepted.
For a leading-hash flag value, use shell quotes: `--colour '#0000ff'`.

Bytes 0 and 3 must be `00` and `01`. The tool preserves those bytes and bytes 5–123.
There are no default key or lighting changes. At least one explicit field is required.

## Human output and script output

Use `--json` for one stable JSON value on stdout. Prompts and diagnostics stay on stderr in JSON mode.
Protocol commands always produce JSON. Human formatting is not a machine-readable contract.

Colour depends on each destination stream, not on stdin. Redirected output, `TERM=dumb`, and non-empty `NO_COLOR` produce plain text.
Terminals that advertise true colour through `COLORTERM=truecolor` or `COLORTERM=24bit` also show a configured RGB swatch beside the exact hex value.
The swatch does not prove the observed LED colour. Output escapes control characters in paths, warnings, and other external text.

| Result | Meaning |
|---|---|
| No changes needed | No settings upload or commit was sent. `set` creates no backup. Legacy `apply` saves one. |
| Cancelled | No settings upload or commit was sent. `set` creates no backup. Legacy `apply` saves one. |
| Dry-run | The device was read. No files or settings were saved. |
| Readback verified | All 128 bytes match intended settings. Persistence after reconnect remains unverified. |
| Failed after submission | Device state is uncertain. Stop. Do not retry automatically. No rollback occurs. |
| Record handling failed | A readback match alone does not establish a durable successful transaction. |

Retention warnings remain separate from transaction failures. Do not retry a successful apply because backup cleanup failed.

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

The advanced `advanced protocol preview` command uses numeric protocol values for all fields.

`plan` and `apply` keep the old numeric settings for existing scripts. They accept `--trigger` values `1` to `3`, `--modifiers` values `0` to `15`, `--rgb-mode` values `0` to `7`, and separate `--red`, `--green`, and `--blue` values `0` to `255`. The numeric-only flags stay hidden from the primary help. Do not combine numeric compatibility values with friendly `--trigger`, `--modifiers`, `--lighting`, or `--colour` values in one command.

For device access, permissions, backup, and restore, see [hardware operations](hardware.md).
For offline parsing and full replacement previews, see the [protocol reference](protocol.md#existing-offline-operations).
