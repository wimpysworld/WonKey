# WonKey documentation

WonKey configures the XFKEY One Key Max (`0112`) and inspects CH57 (`1189:8890`) devices on Linux.
For XFKEY One Key Max writes, it previews changes, asks for confirmation, and verifies all 128 configuration bytes in memory after each write.

[Build WonKey](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started), then [read the current settings](usage#read-settings).
Read [device access requirements](hardware#device-selection-and-access) before a hardware command.

## Use WonKey

| Task | Guide |
|---|---|
| Inspect settings or probe a device | [Read settings](usage#read-settings) |
| Assign a key with modifiers and a trigger | [Set the key](usage#set-the-key) |
| Set a mouse, consumer media, or multi-key action | [Mouse, media, and multi-key actions](usage#mouse-media-and-multi-key-actions) |
| Choose an RGB mode and colour | [Lighting](usage#lighting) |
| Select a device and confirm a change | [Selection and confirmation](usage#selection-and-confirmation) |

The [usage guide](usage) covers command syntax, unified device discovery, and supported settings.
Bare `key`, `mouse`, `media`, `multi`, and `rgb` read settings without modifying the device. Live changes apply only to XFKEY One Key Max (`0112`) and require terminal confirmation.

## Hardware and development

| Page | Use it to |
|---|---|
| [Hardware operations and safety](hardware) | Check device discovery, compatibility, hardware effects, and guarded write verification. |
| [Protocol and evidence](protocol) | Read packet layouts, hardware test results, and pinned source evidence. |
| [Development](development) | Find offline checks, architecture notes, and fixture requirements. |
