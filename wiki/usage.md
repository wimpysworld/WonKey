# Usage and configuration

[Wiki home](Home) · [Hardware operations and safety](hardware)

Build WonKey as described in the [project overview](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run command examples from the project root. Device examples are instructions, not permission to access hardware.

[Read settings](#read-settings) · [Set the key](#set-the-key) · [Mouse, media, and multi-key actions](#mouse-media-and-multi-key-actions) · [Lighting](#lighting) · [Restore](#restore-saved-settings) · [Confirmation](#selection-and-confirmation) · [Backups](#backup-storage)

Changes require a terminal, a preview, and [confirmation](#selection-and-confirmation).
WonKey validates a new durable backup before each settings upload.

## Read settings

```sh
./wonkey key
./wonkey mouse
./wonkey media
./wonkey multi
./wonkey rgb
```

`key` shows a keyboard combination and trigger, or the current typed action. `mouse`, `media`, and `multi` show the current action. `rgb` shows the mode and configured colour.
Each command shows the physical USB path, identifier, and version. They read hardware but create no files.
For an unsupported configuration, bare `key`, `mouse`, `media`, `multi`, and `rgb` instead show an explicit unknown layout.
They show the raw type at byte 0 and configuration bytes 0 to 15 as hexadecimal.
They do not decode key, trigger, lighting, mouse, media, or multi-key fields from an unknown layout.
This read-only display does not enable changes or restore. Both remain blocked before backup or upload for unsupported configurations.
Bare `wonkey`, `wonkey --help`, and help for each command show help without device access.

## Set the key

These commands change stored key settings after confirmation:

```sh
./wonkey key f13
./wonkey key ctrl+shift+f13
./wonkey key ctrl+a
./wonkey key shift+1
./wonkey key f12
./wonkey key ctrl+left
./wonkey key mute
./wonkey key f13 --on release
./wonkey key enter --on press
```

A key expression is the complete combination. `key f13` means **<kbd>F13</kbd> without modifiers**, not <kbd>F13</kbd> with the previous modifiers.

Use exactly one base key from the [supported key names](#supported-key-names).
Names cover letters, digits, function keys, punctuation keys, navigation, keypad keys, and defined keyboard usages through `0x91`.
Use names such as `semicolon` and `keypadplus`, not punctuation symbols or raw usage numbers.
The `key` command does not accept mouse or consumer media names. Macros and right-hand modifiers remain unsupported.
Prefix modifiers with `+`: `ctrl`, `shift`, `alt`, and `super`, each at most once.
`super` is the <kbd>Super</kbd>/<kbd>Windows</kbd>/<kbd>Command</kbd> modifier. Put the base key last. Names ignore letter case.

Uppercase names do not imply <kbd>Shift</kbd>: `A` and `a` select the same key. Use `shift+a` to include <kbd>Shift</kbd>.
Names select USB Human Interface Device (HID) keyboard usages, not guaranteed text. The active host layout and modifiers determine the output.
`mute`, `volumeup`, and `volumedown` select keyboard-page usages, not consumer-page media commands.
`power` selects the keyboard Power usage. The host determines its effect.

`--on` accepts `press`, `release`, or `both`. Omit it to preserve a keyboard trigger. When replacing another action, the default is `press`.
It requires a key expression, so `key --on release` is rejected.
Repeated `--on`, repeated modifiers, unsupported keys, and extra arguments are rejected before device access.
Key changes preserve lighting and unrelated bytes. Conversion to or from a keyboard action with `both` is rejected because separate release fields remain uncertain.

Hardware tests confirmed <kbd>F13</kbd> events and persistence, as recorded in [hardware verification](protocol#hardware-verification).
All other selectable keys remain hardware-unverified. The captured descriptor confirms that the added usages fit, not that firmware executes them.
Desktop keymaps can show different symbols for function keys. Check the key name in your target shortcut editor.

## Mouse, media, and multi-key actions

Each command changes the complete action after confirmation. Omit its action and options to read the current action without a backup.

| Command | Example | Input and limits |
|---|---|---|
| `mouse` | `./wonkey mouse left+right --x=-20 --y 10 --wheel -1` | Use `left`, `right`, `middle`, or a `+` combination without repeats. Use `none` alone for no button, or `wheelup` or `wheeldown` alone for one tick. `--x`, `--y`, and `--wheel` take integers from `-127` to `127`; omitted values are zero. Do not combine a wheel name with `--wheel`. |
| `media` | `./wonkey media playpause` | Use a name below or `0xHHHH` with exactly four hexadecimal digits. The usage must be `0x0001` to `0x023c`. |
| `multi` | `./wonkey multi a,f13,enter --interval 50 --repeat 2` | Use 1 to 115 comma-separated base key names from [supported key names](#supported-key-names). No modifiers or raw usage numbers. `--interval` takes 1 to 65535 milliseconds (default 50). `--repeat` takes 1 to 255 (default 1). |

Media names ignore case: `play`, `pause`, `record`, `fastforward`, `rewind`, `next`, `prev`, `previous`, `stop`, `eject`, `playpause`, `mute`, `volumeup`, `volumedown`, `calculator`, `mycomputer`, `browser`, `email`, `search`, `home`, `back`, `forward`, `refresh`, and `bookmarks`.
`prev` and `previous` select the same usage. A raw usage within the bound does not prove that the device or host supports its effect.
Media, mouse, and multi-key actions replace the complete current action, not only one field.
The `media` command uses HID consumer page `0x0c`. In contrast, `key mute` uses keyboard page `0x07`.

Mouse values are signed relative movement, not pointer positions. `wheelup` means `+1` and `wheeldown` means `-1`.
The multi-key limit stops at configuration byte 119, so bytes 120 to 123 remain unchanged.
The options accept `--name VALUE` or `--name=VALUE` and each option can appear once. Repeated names, invalid values, and extra actions fail before device access.
See [wire layouts and evidence](protocol#action-layouts) for the stored fields. Hardware effects and persistence for these three actions remain unverified.

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
RGB commands preserve the current typed action and unrelated bytes.

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

Restore copies the saved keyboard, mouse, media, or multi-key action and RGB mode and colour into the current configuration.
It preserves unrelated current bytes, including unknown bytes that differ from the capture. It clears the old action's retired active bytes on a type change or shorter sequence.
The preview warns when other saved bytes differ and will remain unchanged. Restore cannot recover firmware, unsupported layouts, or unknown settings changed by another application.
Conversions to or from a keyboard action with `both` are rejected because separate release fields remain uncertain.

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
| Keyboard modifier mask | 2 |
| Keyboard base key (HID usage below) | 4 |
| Lighting mode | 124 |
| RGB channels | 125 to 127 |

Byte 0 selects the action type: `00` keyboard, `01` mouse, `02` media, or `03` multi-key. Keyboard byte 3 must be `01`.
Changes accept only validated actions and RGB modes. WonKey changes active action bytes and explicit RGB fields, and preserves other bytes.
Unknown action types, invalid fields, and unsupported special, macro, or touch layouts fail closed for changes and restore, even for RGB-only changes.
For exact offsets, see [action layouts](protocol#action-layouts).

### Supported key names

Each row lists names in usage order. Ranges include both ends.
Help and previews use these canonical names. Keyboard backup names use them too.

| Base key names | HID usage at byte 4 |
|---|---|
| `a` to `z` | `0x04` to `0x1d` |
| `1` to `9`, `0` | `0x1e` to `0x27` |
| `enter`, `esc`, `backspace`, `tab`, `space` | `0x28` to `0x2c` |
| `minus`, `equal`, `leftbracket`, `rightbracket`, `backslash`, `nonushash` | `0x2d` to `0x32` |
| `semicolon`, `apostrophe`, `grave`, `comma`, `period`, `slash`, `capslock` | `0x33` to `0x39` |
| `f1` to `f12` | `0x3a` to `0x45` |
| `printscreen`, `scrolllock`, `pause`, `insert`, `home`, `pageup` | `0x46` to `0x4b` |
| `delete`, `end`, `pagedown`, `right`, `left`, `down`, `up` | `0x4c` to `0x52` |
| `numlock`, `keypaddivide`, `keypadmultiply`, `keypadminus`, `keypadplus`, `keypadenter` | `0x53` to `0x58` |
| `keypad1` to `keypad9`, `keypad0`, `keypadperiod` | `0x59` to `0x63` |
| `nonusbackslash`, `application`, `power`, `keypadequal` | `0x64` to `0x67` |
| `f13` to `f24` | `0x68` to `0x73` |
| `execute`, `help`, `menu`, `select`, `stop`, `again`, `undo` | `0x74` to `0x7a` |
| `cut`, `copy`, `paste`, `find`, `mute`, `volumeup`, `volumedown` | `0x7b` to `0x81` |
| `lockingcapslock`, `lockingnumlock`, `lockingscrolllock` | `0x82` to `0x84` |
| `keypadcomma`, `keypadequalas400` | `0x85` to `0x86` |
| `international1` to `international9` | `0x87` to `0x8f` |
| `lang1`, `lang2` | `0x90` to `0x91` |

Accepted aliases are `escape` for `esc`, `pgup` for `pageup`, and `pgdn` for `pagedown`.
`arrowright`, `arrowleft`, `arrowdown`, and `arrowup` select `right`, `left`, `down`, and `up`.
No other aliases are accepted. `return` is not an alias because HID also defines a separate Return usage outside this range.

`delete` is forward Delete, not Backspace. `application` and `menu` are distinct usages.
The `locking*` names select distinct locking usages, not the ordinary lock keys.
Keypad effects depend on the host layout and Num Lock state.
The `nonus*`, `international*`, and `lang*` names identify usages without assuming a national layout.

The [USB HID Usage Tables](https://www.usb.org/sites/default/files/hut1_7.pdf), keyboard/keypad page `0x07`, define these values.
The captured keyboard descriptor has a usage maximum of `0x91` and a logical maximum of `0xff`.
WonKey rejects no-event, error, reserved, and out-of-range usages. Descriptor coverage does not prove hardware effects or persistence.
See [protocol evidence](protocol#evidence) for upstream encoding and capture sources.

## Output and options

Human output is not a machine-readable contract. Redirected output, `TERM=dumb`, and non-empty `NO_COLOR` produce plain text.
True-colour terminals also show a configured RGB swatch. The swatch does not prove the observed LED colour.
Output escapes control characters in external text.

Public flags are `-h`/`--help`, `key --on`, `mouse --x`/`--y`/`--wheel`, and `multi --interval`/`--repeat`.
Removed commands and flags, including `show`, `set`, `advanced`, `apply`, `plan`, `--json`, `--yes`, and `--dry-run`, fail before hardware access.
There is one executable, `wonkey`. The developer executable, protocol commands, and helper scripts are removed.
WonKey has no arbitrary packet sender, firmware writer, reset, or bootloader command.
