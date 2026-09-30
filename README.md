# ccs

Full-text search over Claude Code sessions (prompts and Claude's replies, not tool output),
newest first. Enter resumes the session in its own directory.

## Install

```sh
go install github.com/serguacom/ccs@latest
```

Add to `~/.zshrc`:

```zsh
ccs() {
  local out; out="$(command ccs)" || return
  [ -n "$out" ] || return 0
  cd "${out%%$'\t'*}" && claude --resume "${out#*$'\t'}"
}
```

Hidden session IDs are stored in `~/.config/ccs/hidden`, one per line; the file is editable, and
IDs of deleted sessions stay there. Terminals configured to send `0x08` for Backspace (the default
is `0x7f`) trigger hide on Backspace, since that byte decodes as `Ctrl+H`.

`[no dir]` marks sessions whose directory no longer exists; resuming them fails at `cd`.

Current project = sessions started in the current directory. Forked sessions repeat their
parent's history, so a match can appear twice.
