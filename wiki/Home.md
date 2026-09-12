# WonKey documentation

WonKey configures the key combination and RGB lighting of the One Key Max on Linux.
It saves a backup before each change and restores saved settings through the same checks.

[Build WonKey](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started), then [read the current settings](usage#read-settings).
Read [device access requirements](hardware#device-selection-and-access) before a hardware command.

## Use WonKey

| Task | Guide |
|---|---|
| Assign Enter or F13 with modifiers and a trigger | [Set the key](usage#set-the-key) |
| Choose an RGB mode and colour | [Lighting](usage#lighting) |
| Restore settings from an automatic backup or a moved capture | [Restore saved settings](usage#restore-saved-settings) |
| Review and confirm a change | [Selection and confirmation](usage#selection-and-confirmation) |
| Find automatic backups | [Backup storage](usage#backup-storage) |

The [usage guide](usage) covers command syntax and supported settings.
Bare `key` and `rgb` read settings without saved captures. Changes require terminal confirmation.
Restore preserves unrelated current bytes. A settings capture is not a firmware backup.

## Hardware and development

| Page | Use it to |
|---|---|
| [Hardware operations and safety](hardware) | Check compatibility, restore sources, hardware effects, and transaction records. |
| [Protocol and evidence](protocol) | Read packet layouts, hardware test results, and pinned source evidence. |
| [Development](development) | Find offline checks, architecture notes, and fixture requirements. |
