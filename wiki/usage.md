# Usage and configuration

[Wiki home](Home) · [Hardware operations and safety](hardware)

Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run command examples from the project root. Device examples are instructions, not permission to access hardware.

[Read settings](#read-settings) · [Set the key](#set-the-key) · [Lighting](#lighting) · [Restore](#restore-saved-settings) · [Confirmation](#selection-and-confirmation) · [Backups](#backup-storage)

Changes require a terminal, a preview, and [confirmation](#selection-and-confirmation).
WonKey validates a new durable backup before each settings upload.

## Read settings

```sh
./wonkey key
./wonkey rgb
```

`key` shows the current combination and trigger. `rgb` shows the mode and configured colour.
Both show the physical USB path, identifier, and version. They read hardware but create no files.
Bare `wonkey`, `wonkey --help`, and help for each command show help without device access.

## Set the key

These commands change stored key settings after confirmation:

```sh
./wonkey key f13
./wonkey key ctrl+shift+f13
./wonkey key ctrl+a
./wonkey key shift+1
./wonkey key f12
./wonkey key f13 --on release
./wonkey key enter --on press
```

A key expression is the complete combination. `key f13` means **<kbd>F13</kbd> without modifiers**, not <kbd>F13</kbd> with the previous modifiers.

Use exactly one base key: `enter`, `a` to `z`, `0` to `9`, or `f1` to `f24`.
Other base keys, aliases, punctuation, navigation keys, keypad keys, media keys, and macros are unsupported.
Prefix modifiers with `+`: `ctrl`, `shift`, `alt`, and `super`, each at most once.
`super` is the <kbd>Super</kbd>/<kbd>Windows</kbd>/<kbd>Command</kbd> modifier. Put the base key last. Names ignore letter case.

Uppercase names do not imply <kbd>Shift</kbd>: `A` and `a` select the same key. Use `shift+a` to include <kbd>Shift</kbd>.
Letters and digits name HID keys, not guaranteed text. The active host layout and modifiers determine the output.

`--on` accepts `press`, `release`, or `both`. Omit it to preserve the current trigger.
It requires a key expression, so `key --on release` is rejected.
Repeated `--on`, repeated modifiers, unsupported keys, and extra arguments are rejected before device access.
Key changes preserve lighting and all unrelated bytes.

Hardware tests confirmed <kbd>F13</kbd> events and persistence, as recorded in [hardware verification](protocol#hardware-verification).
Letters, digits, <kbd>F1</kbd> to <kbd>F12</kbd>, and <kbd>F14</kbd> to <kbd>F24</kbd> are software-supported but remain unverified on hardware.
Desktop keymaps can show different symbols for function keys. Check the key name in your target shortcut editor.

## Lighting

These commands change stored lighting settings after confirmation:

```sh
./wonkey rgb static 0000ff
./wonkey rgb static
./wonkey rgb off
```

`static`, `breathe`, `flash`, `held`, and `toggle` accept an optional colour: exactly six hexadecimal digits without `#`.
`0000ff` is blue. Omit the colour to preserve all three RGB bytes.
`cycle-slow`, `cycle-fast`, and `off` do not use a colour. They reject supplied colours before device access and preserve the stored colour.
RGB commands preserve the key, modifiers, trigger, and unrelated bytes.

| Mode | Description | Stored byte |
|---|---|---|
| `static` | Single static colour | `02` |
| `breathe` | Single breathing colour | `03` |
| `cycle-slow` | Full-colour slow cycle | `01` |
| `cycle-fast` | Full-colour fast cycle | `05` |
| `flash` | Flash on click | `04` |
| `held` | On while pressed, off on release | `07` |
| `toggle` | Toggle on click | `08` |
| `off` | Lights off | `06` |

Descriptions reflect user observations, not automated hardware verification. See [protocol evidence](protocol#evidence) for the original vendor labels.
Old public names are not accepted. Existing captures remain compatible. WonKey has no brightness or speed option.

## Restore saved settings

These commands restore supported settings after selection and confirmation:

```sh
./wonkey restore
./wonkey restore /path/to/capture-directory
```

Bare `restore` lists compatible captures from automatic storage, newest first. Captures whose supported settings already match are excluded.
Select a capture even when the list contains only one entry. Each entry shows its settings and one short local date and time.
Blank input, EOF, or an invalid capture selection cancels without a backup or settings write.
If no eligible capture exists, WonKey stops without an upload.

An explicit directory selects one existing capture. It can be outside automatic storage, including an old or moved capture.
The source directory does not change where WonKey saves the new backup.
An explicit source whose supported settings already match is a no-op.

Restore copies the saved key, complete modifier combination, trigger, RGB mode, and colour into the current configuration.
It preserves all other current bytes, including unknown bytes that differ from the capture.
Restore cannot recover firmware, unsupported layouts, or unknown settings changed by another application.

The source requires a complete `result.json`, consistent raw replies, and the exact reconstructed configuration.
Both source and current settings must use supported layouts. Model, version, and protocol identifier must match.
A complete backup from a failed transaction remains eligible. Restore does not require a successful apply outcome or retention ownership.
The authentic query fixture lacks a completion record and is not a restore source.

## Selection and confirmation

One compatible device is selected automatically. Multiple matches require a terminal prompt with each physical path and vendor node.
A number selects only the path displayed for this command. It is not a persistent device identifier.
Blank, invalid, out-of-range, or incomplete selection cancels without opening a HID device.
Without a terminal, multiple-device queries and all changes are refused before HID access.

Before a change, WonKey reads current settings and shows current-to-proposed values.
<!-- markdownlint-disable-next-line MD038 -->
At `Save settings? [Y/n]: `, press <kbd>Enter</kbd> to accept the default Yes.
You can also enter `y` or `yes`, ignoring letter case.
`n`, `no`, any other answer, or EOF cancels without a backup or settings write.
There is no bypass flag. A no-op sends no settings upload or commit and creates no backup.

After confirmation, WonKey revalidates the selected descriptors, node, physical path, identifier, and version.
It saves, reopens, and validates a new durable backup before upload. Any settings drift after preview stops the transaction.
This check includes unrelated bytes and drift that makes the request a no-op.

All 128 readback bytes must match. A failure stops without retry or rollback.
A submitted write can still complete after timeout. Stop when the tool reports uncertain state.
Persistence after reconnect and observed key/lighting effects require separate hardware checks.

## Backup storage

| Environment | New backup destination |
|---|---|
| Absolute, non-empty `XDG_STATE_HOME` | `$XDG_STATE_HOME/wonkey/captures` |
| Unset, empty, or relative `XDG_STATE_HOME` | `$HOME/.local/state/wonkey/captures` |
| Fallback with unset, empty, or relative `HOME` | Storage error before upload |

WonKey does not expand a literal `~` or fall back to temporary storage.
It has no custom configuration file or storage flags. `WONKEY_CAPTURE_ROOT` and `XFKEY_CAPTURE_ROOT` have no effect.
Existing captures stay in place. Use an explicit restore source to read one outside automatic storage.
After a successful transaction, retention keeps the newest 10 owned backups for the model and identifier.
Do not retry a successful write because backup cleanup reports a warning.
See [hardware safety and records](hardware#apply-safety-and-records) for durable storage details.

## Supported configuration fields

| Field | Configuration byte |
|---|---|
| Trigger | 1 |
| Complete modifier mask | 2 |
| Base key (HID usage below) | 4 |
| Lighting mode | 124 |
| RGB channels | 125 to 127 |

Bytes 0 and 3 must be `00` and `01`. WonKey preserves those bytes and bytes 5 to 123.
Only known single-key layouts for the supported base keys, trigger values, modifiers, and RGB modes are accepted.
Unknown layouts fail closed, even for RGB-only changes. Macros, mouse/media commands, and multi-key layouts are not converted.

| Base key | HID usage at byte 4 |
|---|---|
| `a` to `z` | `0x04` to `0x1d` |
| `1` to `9` | `0x1e` to `0x26` |
| `0` | `0x27` |
| `enter` | `0x28` |
| `f1` to `f12` | `0x3a` to `0x45` |
| `f13` to `f24` | `0x68` to `0x73` |

The [pinned upstream key table](protocol#evidence) confirms these byte values, not hardware effects or persistence.

## Output and options

Human output is not a machine-readable contract. Redirected output, `TERM=dumb`, and non-empty `NO_COLOR` produce plain text.
True-colour terminals also show a configured RGB swatch. The swatch does not prove the observed LED colour.
Output escapes control characters in external text.

The public flags are `-h`/`--help` and `key --on` only.
Removed commands and flags, including `show`, `set`, `advanced`, `apply`, `plan`, `--json`, `--yes`, and `--dry-run`, fail before hardware access.
There is one executable, `wonkey`. The developer executable, protocol commands, and helper scripts are removed.
WonKey has no arbitrary packet sender, firmware writer, reset, or bootloader command.
