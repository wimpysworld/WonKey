# WonKey documentation

WonKey configures the XFKEY One Key Max (`0112`) and CH57x Macropad (`1189:8890`) devices on Linux.
For XFKEY One Key Max writes, it shows the current-to-proposed diff, checks for settings drift before writing, and verifies all 128 configuration bytes in memory after each write.

[Build WonKey](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started), then [read the current settings](usage#read-settings).
Read [device access requirements](hardware#device-selection-and-access) before a hardware command.

## Use WonKey

| Task | Guide |
|---|---|
| Inspect settings or probe a device | [Read settings](usage#read-settings) |
| Assign a key with modifiers and set its trigger | [Set the key](usage#set-the-key) |
| Set a mouse, consumer media, or macro action | [Mouse, media, and macro actions](usage#mouse-media-and-macro-actions) |
| Choose an RGB or LED lighting mode | [Lighting](usage#lighting) |
| Select a device and verify a change | [Device selection and verification](usage#device-selection-and-verification) |

The [usage guide](usage) covers command syntax, unified device discovery, and supported settings.
Bare `wonkey` reads settings and inspects connected devices without modifying them. `key`, `trigger`, `macro`, `mouse`, `media`, `rgb`, and `led` validate device capabilities (including optional `[CONTROL]` and `--layer <1..15>` on `key`, `macro`, `mouse`, and `media`, and `led <off|reactive|cycle>` on CH57x devices) and configure supported devices (`0112` and `1189:8890`).

## Hardware and development

| Page | Use it to |
|---|---|
| [Hardware operations and safety](hardware) | Check device discovery, compatibility, hardware effects, and guarded write verification. |
| [Protocol and evidence](protocol) | Read packet layouts, hardware test results, and pinned source evidence. |
| [Development](development) | Find offline checks, architecture notes, and fixture requirements. |
