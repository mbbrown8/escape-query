# escq

A command-line tool that reads text and tells you what every terminal
escape sequence in it does.

## Why

Terminal output is full of control bytes that are invisible when things
work and mysterious when they don't. Piping colored program output to a
file, capturing a TUI's raw stream for a bug report, or writing your own
ANSI code by hand all end up with the same problem: a screenful of `^[`
soup that you have to decode by squinting at an ECMA-48 table. `escq`
does that decoding for you and tells you exactly where, in the input,
each sequence starts.

## Usage

Pipe anything containing escape sequences to `escq`, or pass a file:

```
$ printf '\033[1;31mERROR\033[0m: disk full\n' | escq
1:1        ESC[1;31m              bold; set foreground: red
1:10       ESC[0m                 reset all attributes

$ escq captured-session.log
```

Only escape sequences are listed; ordinary text is skipped. Each line is
`line:column  raw-sequence  description`, with the position pointing at
the ESC byte itself.

### Bad input

The point of the tool is to be precise about what's wrong and where.
An unterminated sequence:

```
$ printf 'loading\033[31' | escq
escq: <stdin>: line 1, column 8: unterminated CSI sequence ESC[31 (reached end of input before final byte)
```

is reported with the column of the ESC that started it, not the byte
that finally confused the parser — that's usually the byte you actually
need to go fix.

## What it understands

- CSI sequences (`ESC [ ... letter`): cursor movement, erase, scroll,
  save/restore cursor, SGR (colors and attributes, including 256-color
  and truecolor extended codes), and common DEC private modes (cursor
  visibility, alternate screen buffer, bracketed paste, mouse tracking).
- OSC sequences (`ESC ] ... BEL` or `ESC ] ... ESC \`): window title,
  icon name, hyperlinks, default foreground/background, clipboard access.
- Simple two-byte ESC sequences: save/restore cursor (DEC), full reset,
  index, reverse index, next line, keypad mode.
- Character set designation (`ESC ( X` and friends).

Anything else is reported as an error with its position rather than
silently ignored.

## Build

```
go build -o escq .
```

No third-party dependencies; standard library only.

## Status

Early. Column numbers are byte offsets, not rune offsets, so multi-byte
UTF-8 text before a sequence will throw off the column count slightly.
See the roadmap for what's planned.
