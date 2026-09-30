# ccs

Full-text search over Claude Code sessions (prompts and Claude's replies, not tool output),
newest first. Enter resumes the session in its own directory.

## Install

```sh
go build -o ~/.local/bin/ccs .
```

Add to `~/.zshrc` **after** any `alias claude=...` (zsh expands aliases when the function is defined):

```zsh
ccs() {
  local out; out="$(command ccs)" || return
  [ -n "$out" ] || return 0
  cd "${out%%$'\t'*}" && claude --resume "${out#*$'\t'}"
}
```

## Keys

| Key | Action |
|---|---|
| type | live search (case-insensitive; all words in one message) |
| `↑` `↓` `PgUp` `PgDn` | select |
| `Ctrl+A` | current project ↔ all projects |
| `Tab` | preview transcript |
| `Ctrl+H` | hide / unhide selected session |
| `Ctrl+S` | show hidden sessions ↔ hide them |
| `Enter` | resume |
| `Esc` `Ctrl+Q` `Ctrl+C` | quit |

In preview:

| Key | Action |
|---|---|
| `↑` `↓` `PgUp` `PgDn` | scroll |
| `Ctrl+H` | hide / unhide |
| `Enter` | resume |
| `Esc` `Tab` | back |
| `Ctrl+Q` `Ctrl+C` | quit |

Hidden session IDs are stored in `~/.config/ccs/hidden`, one per line; the file is editable, and
IDs of deleted sessions stay there. Terminals configured to send `0x08` for Backspace (the default
is `0x7f`) trigger hide on Backspace, since that byte decodes as `Ctrl+H`.

`[no dir]` marks sessions whose directory no longer exists; resuming them fails at `cd`.

Current project = sessions started in the current directory. Forked sessions repeat their
parent's history, so a match can appear twice.
