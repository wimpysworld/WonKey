# Usage and configuration

[Wiki index](README.md) · [Project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md)

Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run command examples from the project root. Device examples are instructions, not permission to access hardware.

## Read settings

```sh
./wonkey key
./wonkey rgb
```

`key` shows the current combination and trigger. `rgb` shows the mode and configured colour.
Both show the physical USB path, identifier, and version. They read hardware but create no files.
Bare `wonkey`, `wonkey --help`, `wonkey key --help`, and `wonkey rgb --help` show help without device access.

## Set the key

These commands change stored key settings after confirmation:

```sh
./wonkey key f13
./wonkey key ctrl+shift+f13
./wonkey key f13 --on release
./wonkey key enter --on press
```

A key expression is the complete combination. `key f13` means **F13 without modifiers**, not F13 with the previous modifiers.
The base key must be `enter` or `f13`. Prefix modifiers with `+`: `ctrl`, `shift`, `alt`, and `gui`, each at most once.
`gui` is the Super/Windows/Command modifier. Put the base key last. Names ignore letter case.

`--on` accepts `press`, `release`, or `both`. Omit it to preserve the current trigger.
It requires a key expression, so `key --on release` is rejected.
Repeated `--on`, repeated modifiers, unsupported keys, and extra arguments are rejected before device access.
Key changes preserve lighting and all unrelated bytes.

## Lighting

These commands change stored lighting settings after confirmation:

```sh
./wonkey rgb steady 0000ff
./wonkey rgb steady
./wonkey rgb off
```

The optional colour must contain exactly six hexadecimal digits, with no leading `#`.
`0000ff` is blue. Omit the colour to preserve all three RGB bytes, including when switching to `off`.
RGB commands preserve the key, modifiers, trigger, and unrelated bytes.

| Mode | Vendor interpretation | Stored byte |
|---|---|---|
| `gradient` | Full-colour gradient | `01` |
| `steady` | Single-colour steady | `02` |
| `flowing` | Single-colour flowing | `03` |
| `flash` | Flash on click | `04` |
| `neon` | Neon flowing | `05` |
| `off` | Lights off | `06` |
| `held` | On while pressed, off on release | `07` |
| `toggle` | Toggle on click | `08` |

These names are vendor labels, not verified effects. No brightness or speed option is established.

## Selection and confirmation

One compatible device is selected automatically. Multiple matches require a terminal prompt with each physical path and vendor node.
A number selects only the path displayed for this command. It is not a persistent device identifier.
Blank, invalid, out-of-range, or incomplete selection cancels without opening a HID device.
Without a terminal, multiple-device queries and all changes are refused before HID access.

Before a change, WonKey reads current settings and shows current-to-proposed values.
At `Save settings? [Y/n]:`, press Enter or answer `y` or `yes` to confirm (case-insensitive).
Answer `n` or `no` to cancel. Invalid input or EOF also cancels without a backup or settings write.
There is no bypass flag. A no-op sends no settings upload or commit and creates no backup.

After confirmation, WonKey revalidates the selected descriptors, node, physical path, identifier, and version.
It saves, reopens, and validates a new durable backup before upload. Any settings drift after preview stops the transaction.
This check includes unrelated bytes and drift that makes the request a no-op.

All 128 readback bytes must match. A failure stops without retry or rollback.
A submitted write can still complete after timeout. Stop when the tool reports uncertain state.
Persistence after reconnect and observed key/lighting effects require separate hardware checks.

Backups use the first non-empty value from `WONKEY_CAPTURE_ROOT`, `XFKEY_CAPTURE_ROOT`, and `$HOME/.local/state/xfkey-captures`.
An environment override must be an absolute path. No command-line override is accepted.
After a successful transaction, retention keeps the newest 10 owned backups for the model and identifier.
Do not retry a successful write because backup cleanup reports a warning.
See [hardware safety and records](hardware.md#apply-safety-and-records) for durable storage details.

## Supported configuration fields

| Field | Configuration byte |
|---|---|
| Trigger | 1 |
| Complete modifier mask | 2 |
| Enter (`28`) or F13 (`68`) | 4 |
| Lighting mode | 124 |
| RGB channels | 125–127 |

Bytes 0 and 3 must be `00` and `01`. WonKey preserves those bytes and bytes 5–123.
Only known single-key Enter/F13 layouts, trigger values, modifiers, and RGB modes are accepted.
Unknown layouts fail closed, even for RGB-only changes. Macros, mouse/media commands, and multi-key layouts are not converted.

## Output and developer interfaces

Human output is not a machine-readable contract. Redirected output, `TERM=dumb`, and non-empty `NO_COLOR` produce plain text.
True-colour terminals also show a configured RGB swatch. The swatch does not prove the observed LED colour.
Output escapes control characters in external text.

The public flags are `-h`/`--help` and `key --on` only.
Removed commands and flags, including `show`, `set`, `advanced`, `apply`, `plan`, `--json`, `--yes`, and `--dry-run`, fail before hardware access.
Use the separately built [developer tools](development.md) for saved-capture analysis, protocol parsing, and legacy scripts.
