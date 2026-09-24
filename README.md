<h1 align="center">
  <img src="pages/icon.svg" width="256" height="256" alt="WonKey">
  <br />
  WonKey
</h1>

<p align="center"><b>Configure your XFKEY One Key Max on Linux. One key, to rule them all 1️⃣</b></p>

<p align="center">Made with 💝 for 🐧</p>

Set a keyboard, relative mouse, consumer media, or multi-key action, choose RGB lighting, and restore saved settings.

WonKey previews changes, asks for confirmation, and validates an automatic backup before it writes settings.

Supports only the verified descriptor profile for model `0112`. Keyboard actions use one base key with optional <kbd>Ctrl</kbd>, <kbd>Shift</kbd>, <kbd>Alt</kbd>, and <kbd>Super</kbd> modifiers.
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

The build replaces `./bin/wonkey` in the source directory.

### Check the installation

```sh
wonkey --help
```

Help commands do not access the device.
Your user needs permission to access the device's vendor hidraw node. WonKey does not install permission rules.

## Make it yours

Read the current settings without changing them or creating files:

```sh
wonkey key
wonkey mouse
wonkey media
wonkey multi
wonkey rgb
```

Set the key combination:

```sh
wonkey key f13
wonkey key ctrl+shift+a
wonkey key 0
wonkey key f12
wonkey key f13 --on release
```

A key expression is the **complete combination**. `key f13` clears all modifiers.
Omit `--on` to preserve a keyboard trigger. When replacing another action, the default is `press`.

Names ignore letter case. `A` means the same key as `a`, without an implied <kbd>Shift</kbd>.
Letters and digits name HID keys, not guaranteed text. The active host layout and modifiers determine the output.

Set a mouse movement, a consumer media action, or a key sequence:

```sh
wonkey mouse left+right --x -20 --y 10
wonkey mouse wheelup
wonkey media playpause
wonkey media 0x00e9
wonkey multi a,f13,enter --interval 50 --repeat 2
```

Mouse coordinates and wheel ticks are relative; omitted values are zero. The multi-key defaults are 50 milliseconds and one repetition.
See [action syntax and limits](wiki/usage.md#mouse-media-and-multi-key-actions) before a change.

Choose static blue or turn the lighting off:

```sh
wonkey rgb static 0000ff
wonkey rgb off
```

`static`, `breathe`, `flash`, `held`, and `toggle` accept an optional colour: six hexadecimal digits without `#`.
Omit the colour to preserve it. `cycle-slow`, `cycle-fast`, and `off` do not use a colour and reject supplied colours.
RGB commands preserve key settings.

Choose a compatible backup and restore its settings:

```sh
wonkey restore
```

Restore copies the saved keyboard, mouse, media, or multi-key action and RGB mode and colour. It preserves unrelated current configuration bytes.
Restore does not recover firmware.

## Before you save

Changes require a terminal. WonKey shows the selected device and current-to-proposed settings before it asks `Save settings? [Y/n]:`.
Press <kbd>Enter</kbd> to save, or enter `n` to cancel. Cancellation and unchanged settings create no backup and send no settings writes.

If backup validation fails or the device state changes after the preview, WonKey stops before it writes settings.
After a write, it compares all 128 configuration bytes. Errors stop the transaction without automatic retries or rollback.

## Compatibility

WonKey checks exact device descriptors as well as the model. It rejects unsupported special, macro, touch, and unknown layouts.
The `key` names `mute`, `volumeup`, and `volumedown` use keyboard-page usages. The `media` command uses consumer-page usages. Right-hand modifiers are unsupported.

Hardware checks on one unit confirmed <kbd>F13</kbd> press/release events, steady-blue lighting, and persistence of both settings after reconnect.
All other selectable keys and the new mouse, media, and multi-key actions remain hardware-unverified. Descriptor coverage does not prove firmware execution.
Other devices and descriptor profiles remain unsupported.
Configuration readback alone does not prove physical effects or persistence.

## Documentation

See the [Wiki](wiki/README.md) for all options, backup storage, hardware guidance, and development documentation.
