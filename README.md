# VaultUI

A secure, minimal, and interactive TUI note-taking application built in Go.

![VaultUI](https://via.placeholder.com/800x400.png?text=VaultUI+TUI+Screenshot) *(Placeholder for screenshot)*

VaultUI manages multiple "notebooks" (vaults), encrypts notes on a per-notebook basis, and seamlessly integrates with your favorite terminal text editor (`micro`, `nano`, `vim`, etc.) to write and edit notes, all while keeping your data securely encrypted at rest.

## Features

- **Interactive TUI**: Built with Bubble Tea for a snappy and beautiful terminal interface.
- **Secure Encryption**: Uses `AES-256-GCM` for encrypting your notes and `Argon2` for secure key derivation from your vault passwords.
- **Per-Notebook Security**: Each notebook has its own password. You unlock it once per session, and the key is securely wiped from memory the moment you exit the notebook.
- **Native Editor Support**: Temporarily decrypts notes to a RAM-friendly secure file, allowing your native `$EDITOR` (defaults to `micro` or `nano`) to provide syntax highlighting and normal editing flows, before securely re-encrypting upon save.
- **Passwordless Vaults**: Need a quick scratchpad? Just hit Enter when creating a vault to create an unencrypted/passwordless notebook.
- **Archive & Management**: Easily archive or permanently delete notebooks directly from the TUI.

## Install

### From source

```bash
# Clone the repository
git clone git@github.com:mohamed-el-bouchir/vaultui.git
cd vaultui

# Build the binary
go build -o vaultui

# Move to your bin path
sudo mv vaultui /usr/local/bin/
```

### Via Go Install

```bash
go install github.com/mohamed-el-bouchir/vaultui@latest
```

## Usage

Simply run the application:

```bash
vaultui
```

### Global Shortcuts

- **`↑/k`** and **`↓/j`**: Navigate lists
- **`Enter`**: Select/Open/Confirm
- **`Esc`**: Go back, cancel, or securely lock the current vault and clear session memory
- **`Ctrl+c`**: Quit application

### Notebooks View Shortcuts

When viewing your list of notebooks:

- **`s`**: Open **Settings** to change the default vault location
- **`d`**: **Delete** the highlighted notebook (requires password confirmation)
- **`a`**: **Archive** the highlighted notebook (moves to a hidden `.archive` folder)
- **`c`**: **Change Password** for the highlighted notebook (requires old password, re-encrypts all notes with the new one)

### Notes View Shortcuts

When inside an unlocked notebook:

- **`n`**: Create a **New Note**
- **`Enter`**: Open the highlighted note in your `$EDITOR`
- **`Esc`**: Lock the vault and return to the notebooks list

## Configuration & Data Storage

- **Configuration**: Stored at `~/.config/vaultui/config.toml`. You can configure your `vault_root` directory and default `editor` here.
- **Data Storage**: By default, notebooks are stored in `~/.vaulted/`.
- Each notebook directory contains a `.vault_meta` file which securely stores the Argon2 salt and password validation hash.
- Encrypted notes have a `.enc` extension, but are transparently opened as `.md` files in your editor to support markdown highlighting.

## Security Architecture

1. **Authentication**: VaultUI derives a 32-byte key using Argon2id and the salt from `.vault_meta`. It hashes the key via SHA256 and compares it to the validation hash.
2. **Session Memory**: If valid, the AES key is kept in memory. The moment you navigate back (`Esc`), the slice is explicitly zeroed out to prevent memory dumping.
3. **Editor Handoff**: Decrypted content is written to a temporary file (`os.CreateTemp` with strict `0600` permissions). The TUI pauses, spawning the editor. On exit, the file is re-encrypted, and the temporary plaintext file is immediately destroyed via `os.Remove`.

## Upcoming Features

- More coming soon...

## License

MIT License - Mohamed EL BOUCHIR
