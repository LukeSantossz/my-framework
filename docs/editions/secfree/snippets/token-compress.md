- `caveman-compress`: would rewrite `CLAUDE.md` into a compressed form, cutting
  input tokens every session while preserving code, URLs, and paths byte-for-byte.
  The installed Caveman does not have it. The skill is a conversational mode: it
  never mentions `CLAUDE.md`, context files, or byte-for-byte preservation, and
  nothing in it rewrites a file. `caveman-compress` is therefore a capability this
  policy scopes for the day it arrives, not one the framework can invoke now.
