# WonKey documentation

WonKey reads and changes key combinations and RGB settings on a descriptor-verified device on Linux.
Start with the [project overview and build instructions](https://github.com/wimpysworld/WonKey/blob/main/README.md#get-started).
Run every command example from the project root, not from `wiki/`.

| Page | Use it to |
|---|---|
| [Usage and configuration](usage.md) | Set a key combination, choose an RGB mode, or restore saved settings. |
| [Development](development.md) | Check the build, architecture, and evidence fixtures. |
| [Hardware operations and safety](hardware.md) | Understand device selection, backup safeguards, and restore limits. |
| [Protocol and evidence](protocol.md) | Check reply encodings, captured identifiers, and source evidence. |

`key COMBINATION`, `rgb MODE [RGB]`, and `restore [capture-directory]` change stored settings after confirmation.
Bare `key` and `rgb` only read settings.
Read the hardware safety conditions before any live operation.
Configuration is not firmware flashing. Captures prove neither upload effects nor persistence.

For development and offline checks, read [AGENTS.md](https://github.com/wimpysworld/WonKey/blob/main/AGENTS.md).
