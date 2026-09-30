# search: wrap counts runes, not display cells

`search.wrap` measures line width in runes. Wide characters (CJK, emoji) make a wrapped
line exceed the terminal width in cells. In the list this is hidden by `ansi.Truncate`
(`…`); in preview the viewport hard-cuts the line (`ansi.Cut`) so the tail is silently lost.

Fix: measure with `ansi.StringWidth` / `github.com/rivo/uniseg` cell widths in `wrap`,
and adjust `Highlight` ranges (rune-indexed) accordingly.
