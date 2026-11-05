<h1 align="center">
  <img src="pages/icon.svg" width="256" height="256" alt="WonKey">
  <br />
  WonKey
</h1>

<p align="center"><b>Configure your XFKEY One Key Max on Linux. One key, to rule them all 1️⃣</b></p>

<p align="center">Made with 💝 for 🐧</p>

Set a keyboard, relative mouse, consumer media, or macro action, and choose RGB or LED lighting.

WonKey previews changes, asks for confirmation, and writes settings to XFKEY One Key Max (`0112`) and CH57x Macropad (`1189:8890`) devices.

WonKey discovers compatible XFKEY One Key Max (`0112`) and CH57x Macropad (`1189:8890`) devices automatically. Bare `wonkey` inspects all connected XFKEY One Key Max (`0112`) and CH57x Macropad (`1189:8890`) devices.
Keyboard actions use one base key with optional <kbd>Ctrl</kbd>, <kbd>Shift</kbd>, <kbd>Alt</kbd>, and <kbd>Super</kbd> modifiers.
Current source supports letters, digits, function keys, navigation, punctuation names, keypad keys, and other defined keyboard usages through `0x91`.
See [supported key names and aliases](wiki/usage.md#supported-key-names).

## Get started

WonKey 0.1.1 is available for Linux. Download a package from the [release](https://github.com/wimpysworld/WonKey/releases/tag/v0.1.1), then run its command from the download directory.
The new action commands and expanded key names describe the current source, not the 0.1.1 release. Build this checkout to use them.
For ARM64, replace `amd64` in the x86_64 examples with `arm64`.

| Distribution | Package | Install |
| --- | --- | --- |
| Debian / Ubuntu | `.deb` | `sudo apt install ./wonkey_0.1.1_linux_amd64.deb` |
| Fedora | `.rpm` | `sudo dnf install ./wonkey_0.1.1_linux_amd64.rpm` |
| openSUSE | `.rpm` | `sudo zypper install ./wonkey_0.1.1_linux_amd64.rpm` |
| Alpine | `.apk` | `sudo apk add --allow-untrusted ./wonkey_0.1.1_linux_amd64.apk` |
| Arch Linux | [AUR](https://aur.archlinux.org/packages/xfkey-wonkey-bin) | `yay -S xfkey-wonkey-bin` (requires `yay`) |

The Alpine package is unsigned, so `apk` requires `--allow-untrusted`.
Compare downloads with the [SHA-256 checksums](https://github.com/wimpysworld/WonKey/releases/download/v0.1.1/WonKey_0.1.1_checksums.txt) before installation.

### Binary archive

Download the archive for [x86_64](https://github.com/wimpysworld/WonKey/releases/download/v0.1.1/WonKey_0.1.1_linux_amd64.tar.gz) or [ARM64](https://github.com/wimpysworld/WonKey/releases/download/v0.1.1/WonKey_0.1.1_linux_arm64.tar.gz), then extract and install it:

```sh
tar -xzf WonKey_0.1.1_linux_amd64.tar.gz
sudo install -m 0755 wonkey /usr/local/bin/wonkey
```

### Build from source

Requires Git, Go 1.25 or later, and [just](https://just.systems/).

```sh
# From a checkout that contains the current source:
just build
sudo install -m 0755 ./bin/wonkey /usr/local/bin/wonkey
```

The build replaces `./bin/wonkey` and `./wonkey` in the source directory.

### Check the installation

```sh
wonkey --help
```

Help commands do not access the device.
Your user needs permission to access the XFKEY vendor hidraw node or the CH57 USB device node. WonKey does not install permission rules.

## Make it yours

Read the current settings without changing them:

```sh
wonkey
```

Bare `wonkey` enumerates and inspects all compatible XFKEY One Key Max (`0112`) and CH57x Macropad (`1189:8890`) devices automatically, sorted by USB physical path with numbered headings such as `[1] XFKEY One Key Max (0112)` and `[2] CH57x Macropad (1189:8890)`.
On each XFKEY One Key Max (`0112`), `wonkey` shows its USB path, supported commands (`key, trigger, mouse, media, macro, rgb`), configured action, and RGB lighting.
On each CH57x Macropad (`1189:8890`), `wonkey` shows its USB path, supported commands (`key, mouse, media, macro, led`), and `Settings   Write-only (not readable over USB)` (plus `Input hex` when a 65-byte report ID 3 reply is returned). Stock write-only `1189:8890` firmware returns a 1-byte `0xaa` status reply, which WonKey accepts alongside 65-byte report ID 3 replies.

Set the key combination and keyboard trigger:

```sh
wonkey key f13
wonkey key ctrl+shift+a
wonkey key 0
wonkey key f12
wonkey key key2 f13
wonkey trigger release
```

A key expression is the **complete combination**. `key f13` clears all modifiers and preserves an existing keyboard trigger (or defaults to `press` when replacing another action).
Use `wonkey trigger <press|release|both>` on XFKEY One Key Max (`0112`) to change when a configured keyboard key fires.
`key`, `macro`, `mouse`, and `media` accept an optional `[CONTROL]` (`key1`..`key12`, `knob-left`, `knob-press`, `knob-right`, `knob1-left`..`knob3-right`, defaulting to `key1`) before the action argument and an optional `--layer <1..15>` flag (defaulting to `1`) on multi-control CH57x (`1189:8890`) devices. Single-key XFKEY One Key Max (`0112`) devices reject `[CONTROL]` and `--layer` with a capability error.

Names ignore letter case. `A` means the same key as `a`, without an implied <kbd>Shift</kbd>.
Letters and digits name HID keys, not guaranteed text. The active host layout and modifiers determine the output.

Set a mouse movement, a consumer media action, or a macro key sequence:

```sh
wonkey mouse left+right --x -20 --y 10
wonkey mouse wheelup
wonkey media playpause
wonkey media knob-right volumeup
wonkey media 0x00e9
wonkey macro a,f13,enter --interval 50 --repeat 2
wonkey macro key3 ctrl+c,ctrl+v --layer 2
```

Mouse coordinates and wheel ticks are relative; omitted values are zero. On `0112`, the `macro` defaults are 50 milliseconds and one repetition for 1 to 115 base keys; `1189:8890` supports 1 to 5 strokes including per-stroke modifiers such as `ctrl+c,ctrl+v`.
See [action syntax and limits](wiki/usage.md#mouse-media-and-macro-actions) before a change.

Choose RGB lighting on `0112` or LED backlight mode on `1189:8890`:

```sh
wonkey rgb static 0000ff
wonkey rgb off
wonkey led static
wonkey led off
```

`static`, `breathe`, `flash`, `held`, and `toggle` accept an optional colour on `0112`: six hexadecimal digits without `#`.
Omit the colour to preserve it. `cycle-slow`, `cycle-fast`, and `off` do not use a colour and reject supplied colours.
`wonkey rgb` is supported on `0112` only (rejected on `1189:8890` with a hint to use `wonkey led`).
`wonkey led` accepts `off`, `on`, `static`, or `0..255` on `1189:8890` (rejected on `0112` with a hint to use `wonkey rgb`).
Lighting commands preserve key settings.

## Before you save

Changes require a terminal and a supported device (`0112` or `1189:8890`). When one compatible device is attached, WonKey selects it automatically; when multiple compatible devices are attached, pass `--device <N>` using the USB-path-sorted device number from `wonkey` (for example `wonkey key f13 --device 1`). WonKey shows the selected device heading (such as `[1] XFKEY One Key Max (0112)` or `[2] CH57x Macropad (1189:8890)`) and current-to-proposed settings before it asks `Save settings? [Y/n]:`.
Press <kbd>Enter</kbd> to save, or enter `n` to cancel. Cancellation and unchanged settings send no settings writes.

On `0112`, if the device state changes after the preview, WonKey stops before it writes settings, and after an `AF02` upload and `AF04` commit, it reads back and compares all 128 configuration bytes in memory. On `1189:8890`, WonKey revalidates the USB descriptors and interface 1 report descriptor before writing the 64-byte zero initialisation packet and encoded report sequence to interrupt OUT endpoint `0x02`. Errors stop the transaction without automatic retries or rollback.

## Compatibility

WonKey checks exact device descriptors for XFKEY One Key Max (`af88:6688`, model `0112`) and CH57x Macropad (`1189:8890`). For `0112`, it rejects unsupported special, macro (`06`), touch, and unknown layouts, rejects `[CONTROL]` and `--layer` (`XFKEY One Key Max (0112) has a single key and does not support ...` / `XFKEY One Key Max (0112) does not support --layer`), and rejects `wonkey led` with a hint to use `wonkey rgb`. For CH57x Macropad (`1189:8890`), WonKey checks that the requested command and options fit CH57x hardware capabilities (optional `[CONTROL]` in `key1`..`key12`, `knob-left`, `knob-press`, `knob-right`, `knob1-left`..`knob3-right` defaulting to `key1`, `--layer <1..15>` defaulting to `1`, `a-z`, `0-9`, and `f1-f24` usages `0x04..0x73`, no `trigger` command, `macro` up to 5 strokes with optional per-stroke modifiers and without `--interval` or `--repeat`, CH57x-supported consumer media usages, and `led` modes `off`, `on`, `static`, or `0..255` instead of `rgb`) before previewing and writing the settings.
The `key` names `mute`, `volumeup`, and `volumedown` use keyboard-page usages. The `media` command uses consumer-page usages. Right-hand modifiers are unsupported.

Hardware checks on one `0112` unit confirmed <kbd>F13</kbd> press/release events, steady-blue lighting, and persistence of both settings after reconnect.
All other selectable keys and the new mouse, media, and macro actions remain hardware-unverified. Descriptor coverage does not prove firmware execution.
Other devices and descriptor profiles remain unsupported.
Configuration readback alone does not prove physical effects or persistence.

## Documentation

See the [Wiki](wiki/README.md) for all options, hardware guidance, and development documentation.
