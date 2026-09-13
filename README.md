# WonKey 1️⃣

One key. Your rules.

Configure your XFKEY One Key Max on Linux. Set the key combination, choose the RGB lighting, and restore saved settings.

WonKey previews changes, asks for confirmation, and validates an automatic backup before it writes settings.

Supports model `0112` with one base key: `enter`, `a` to `z`, `0` to `9`, or `f1` to `f24`.
Add optional <kbd>Ctrl</kbd>, <kbd>Shift</kbd>, <kbd>Alt</kbd>, and <kbd>Super</kbd> modifiers.

## Get started

Requires Linux, Go 1.23 or later, and [just](https://just.systems/). Run commands from the project root.

```sh
just build
./wonkey --help
```

The build replaces `./wonkey`. Help commands do not access the device.
Your user needs permission to access the device's vendor hidraw node. WonKey does not install permission rules.

## Make it yours

Read the current settings without changing them or creating files:

```sh
./wonkey key
./wonkey rgb
```

Set the key combination:

```sh
./wonkey key f13
./wonkey key ctrl+shift+a
./wonkey key 0
./wonkey key f12
./wonkey key f13 --on release
```

A key expression is the **complete combination**. `key f13` clears all modifiers.
Omit `--on` to preserve the current trigger.

Names ignore letter case. `A` means the same key as `a`, without an implied <kbd>Shift</kbd>.
Letters and digits name HID keys, not guaranteed text. The active host layout and modifiers determine the output.

Choose steady blue or turn the lighting off:

```sh
./wonkey rgb steady 0000ff
./wonkey rgb off
```

Omit the six-digit RGB value to preserve the colour. RGB commands preserve key settings.

Choose a compatible backup and restore its settings:

```sh
./wonkey restore
```

Restore changes only the saved key, modifiers, trigger, RGB mode, and colour. It preserves all other current configuration bytes.
Restore does not recover firmware.

## Before you save

Changes require a terminal. WonKey shows the selected device and current-to-proposed settings before it asks `Save settings? [Y/n]: `.
Press <kbd>Enter</kbd> to save, or enter `n` to cancel. Cancellation and unchanged settings create no backup and send no settings writes.

If backup validation fails or the device state changes after the preview, WonKey stops before it writes settings.
After a write, it compares all 128 configuration bytes. Errors stop the transaction without automatic retries or rollback.

## Compatibility

WonKey checks exact device descriptors as well as the model. It rejects unsupported layouts, including macros and mouse or media commands.

Hardware checks on one unit confirmed <kbd>F13</kbd> press/release events, steady-blue lighting, and persistence of both settings after reconnect.
Letters, digits, <kbd>F1</kbd> to <kbd>F12</kbd>, and <kbd>F14</kbd> to <kbd>F24</kbd> are software-supported but remain unverified on hardware.
Other settings and devices also remain unverified.
Configuration readback alone does not prove physical effects or persistence.

## Documentation

See the [Wiki](wiki/README.md) for all options, backup storage, hardware guidance, and development documentation.
