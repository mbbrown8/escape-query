package main

import (
	"fmt"
	"strings"
)

const (
	esc = 0x1B
	bel = 0x07
)

// Pos is a 1-based line/column position in the scanned input. Columns are
// counted in bytes, not runes, so a sequence following multi-byte UTF-8
// text will report a byte offset rather than a character offset.
type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

// Sequence is one decoded escape sequence found in the input.
type Sequence struct {
	Pos         Pos
	Raw         string
	Description string
}

// ScanError describes a malformed or unrecognized escape sequence, anchored
// to the position of the ESC byte that started it.
type ScanError struct {
	Pos Pos
	Msg string
}

func (e *ScanError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
}

// Scan walks data looking for terminal escape sequences. It returns a
// decoded Sequence for each one it recognizes, in order, plus one error
// per sequence that is malformed or runs off the end of the input before
// it is terminated. A scan error stops interpretation of that sequence
// but not the rest of the input.
func Scan(data []byte) ([]Sequence, []error) {
	var seqs []Sequence
	var errs []error

	line, col := 1, 1
	i := 0

	advance := func(n int) {
		for j := 0; j < n; j++ {
			if data[i+j] == '\n' {
				line++
				col = 1
			} else {
				col++
			}
		}
		i += n
	}

	for i < len(data) {
		if data[i] != esc {
			advance(1)
			continue
		}

		start := Pos{Line: line, Col: col}

		if i+1 >= len(data) {
			errs = append(errs, &ScanError{Pos: start, Msg: "lone ESC at end of input"})
			advance(1)
			continue
		}

		switch data[i+1] {
		case '[':
			seq, n, err := scanCSI(data[i:], start)
			if err != nil {
				errs = append(errs, err)
				advance(len(data) - i)
				continue
			}
			seqs = append(seqs, seq)
			advance(n)
		case ']':
			seq, n, err := scanOSC(data[i:], start)
			if err != nil {
				errs = append(errs, err)
				advance(len(data) - i)
				continue
			}
			seqs = append(seqs, seq)
			advance(n)
		default:
			seq, n, err := scanSimple(data[i:], start)
			if err != nil {
				errs = append(errs, err)
				advance(2)
				continue
			}
			seqs = append(seqs, seq)
			advance(n)
		}
	}

	return seqs, errs
}

// scanCSI parses a Control Sequence Introducer: ESC [ params intermediates final.
// Parameter bytes are 0x30-0x3F, intermediate bytes 0x20-0x2F, and the
// sequence ends at the first byte in 0x40-0x7E.
func scanCSI(data []byte, start Pos) (Sequence, int, error) {
	i := 2
	for i < len(data) {
		b := data[i]
		switch {
		case b >= 0x30 && b <= 0x3F:
			i++
		case b >= 0x20 && b <= 0x2F:
			i++
		case b >= 0x40 && b <= 0x7E:
			return Sequence{
				Pos:         start,
				Raw:         escapeVisual(string(data[:i+1])),
				Description: describeCSI(data[2:i], b),
			}, i + 1, nil
		default:
			return Sequence{}, 0, &ScanError{
				Pos: start,
				Msg: fmt.Sprintf("invalid byte 0x%02X in CSI sequence %s", b, escapeVisual(string(data[:i+1]))),
			}
		}
	}
	return Sequence{}, 0, &ScanError{
		Pos: start,
		Msg: fmt.Sprintf("unterminated CSI sequence %s (reached end of input before final byte)", escapeVisual(string(data))),
	}
}

func describeCSI(params []byte, final byte) string {
	p := string(params)
	switch final {
	case 'A':
		return "cursor up " + countOrDefault(p, "1")
	case 'B':
		return "cursor down " + countOrDefault(p, "1")
	case 'C':
		return "cursor forward " + countOrDefault(p, "1")
	case 'D':
		return "cursor back " + countOrDefault(p, "1")
	case 'E':
		return "cursor to start of line, " + countOrDefault(p, "1") + " line(s) down"
	case 'F':
		return "cursor to start of line, " + countOrDefault(p, "1") + " line(s) up"
	case 'G':
		return "cursor to column " + countOrDefault(p, "1")
	case 'H', 'f':
		return "cursor to " + cursorPos(p)
	case 'J':
		return "erase in display: " + eraseMode(p)
	case 'K':
		return "erase in line: " + eraseMode(p)
	case 'S':
		return "scroll up " + countOrDefault(p, "1") + " line(s)"
	case 'T':
		return "scroll down " + countOrDefault(p, "1") + " line(s)"
	case 's':
		return "save cursor position"
	case 'u':
		return "restore cursor position"
	case 'n':
		if p == "6" {
			return "request cursor position report"
		}
		return "device status report request (code " + p + ")"
	case 'm':
		return describeSGR(p)
	case 'h':
		return describePrivateMode(p, true)
	case 'l':
		return describePrivateMode(p, false)
	default:
		return fmt.Sprintf("CSI sequence, final byte %q (unrecognized)", string(final))
	}
}

func countOrDefault(p, def string) string {
	if p == "" {
		return def
	}
	return p
}

func cursorPos(p string) string {
	row, col := "1", "1"
	if p != "" {
		parts := strings.Split(p, ";")
		if len(parts) > 0 && parts[0] != "" {
			row = parts[0]
		}
		if len(parts) > 1 && parts[1] != "" {
			col = parts[1]
		}
	}
	return fmt.Sprintf("row %s, column %s", row, col)
}

func eraseMode(p string) string {
	switch p {
	case "", "0":
		return "from cursor to end"
	case "1":
		return "from start to cursor"
	case "2":
		return "entire display/line"
	case "3":
		return "entire display, including scrollback"
	default:
		return "mode " + p + " (unrecognized)"
	}
}

var sgrNames = map[string]string{
	"0": "reset all attributes", "1": "bold", "2": "faint", "3": "italic",
	"4": "underline", "5": "slow blink", "6": "rapid blink", "7": "reverse video",
	"8": "conceal", "9": "strikethrough",
	"22": "normal intensity", "23": "not italic", "24": "not underlined",
	"25": "not blinking", "27": "not reversed", "28": "reveal", "29": "not strikethrough",
	"30": "set foreground: black", "31": "set foreground: red", "32": "set foreground: green",
	"33": "set foreground: yellow", "34": "set foreground: blue", "35": "set foreground: magenta",
	"36": "set foreground: cyan", "37": "set foreground: white", "39": "default foreground",
	"40": "set background: black", "41": "set background: red", "42": "set background: green",
	"43": "set background: yellow", "44": "set background: blue", "45": "set background: magenta",
	"46": "set background: cyan", "47": "set background: white", "49": "default background",
	"90": "set foreground: bright black", "91": "set foreground: bright red",
	"92": "set foreground: bright green", "93": "set foreground: bright yellow",
	"94": "set foreground: bright blue", "95": "set foreground: bright magenta",
	"96": "set foreground: bright cyan", "97": "set foreground: bright white",
	"100": "set background: bright black", "101": "set background: bright red",
	"102": "set background: bright green", "103": "set background: bright yellow",
	"104": "set background: bright blue", "105": "set background: bright magenta",
	"106": "set background: bright cyan", "107": "set background: bright white",
}

func describeSGR(p string) string {
	if p == "" {
		return "reset all attributes"
	}
	parts := strings.Split(p, ";")
	var descs []string
	for i := 0; i < len(parts); i++ {
		code := parts[i]
		if code == "38" || code == "48" {
			ground := "foreground"
			if code == "48" {
				ground = "background"
			}
			if i+2 < len(parts) && parts[i+1] == "5" {
				descs = append(descs, fmt.Sprintf("set %s: 256-color palette index %s", ground, parts[i+2]))
				i += 2
				continue
			}
			if i+4 < len(parts) && parts[i+1] == "2" {
				descs = append(descs, fmt.Sprintf("set %s: rgb(%s, %s, %s)", ground, parts[i+2], parts[i+3], parts[i+4]))
				i += 4
				continue
			}
			descs = append(descs, fmt.Sprintf("set %s color (malformed extended color code)", ground))
			continue
		}
		if name, ok := sgrNames[code]; ok {
			descs = append(descs, name)
		} else {
			descs = append(descs, "attribute "+code+" (unrecognized)")
		}
	}
	return strings.Join(descs, "; ")
}

var privateModeNames = map[string]string{
	"1":    "application cursor keys",
	"12":   "cursor blinking",
	"25":   "cursor visibility",
	"1000": "mouse click tracking",
	"1002": "mouse button-event tracking",
	"1006": "SGR extended mouse mode",
	"1049": "alternate screen buffer",
	"2004": "bracketed paste mode",
}

func describePrivateMode(p string, enable bool) string {
	action := "disable"
	if enable {
		action = "enable"
	}
	if !strings.HasPrefix(p, "?") {
		return fmt.Sprintf("%s mode(s) %s (unrecognized)", action, p)
	}
	codes := strings.Split(strings.TrimPrefix(p, "?"), ";")
	var descs []string
	for _, code := range codes {
		if name, ok := privateModeNames[code]; ok {
			descs = append(descs, name)
		} else {
			descs = append(descs, code+" (unrecognized)")
		}
	}
	return fmt.Sprintf("%s: %s", action, strings.Join(descs, ", "))
}

// scanOSC parses an Operating System Command: ESC ] ... terminated by
// either BEL or the two-byte String Terminator ESC \.
func scanOSC(data []byte, start Pos) (Sequence, int, error) {
	i := 2
	for i < len(data) {
		if data[i] == bel {
			return buildOSC(data[:i+1], start), i + 1, nil
		}
		if data[i] == esc {
			if i+1 < len(data) && data[i+1] == '\\' {
				return buildOSC(data[:i+2], start), i + 2, nil
			}
			return Sequence{}, 0, &ScanError{
				Pos: start,
				Msg: fmt.Sprintf("unterminated OSC sequence %s (ESC not followed by backslash)", escapeVisual(string(data[:i+1]))),
			}
		}
		i++
	}
	return Sequence{}, 0, &ScanError{
		Pos: start,
		Msg: fmt.Sprintf("unterminated OSC sequence %s (reached end of input, expected BEL or ESC \\)", escapeVisual(string(data))),
	}
}

func buildOSC(raw []byte, start Pos) Sequence {
	body := raw[2:]
	switch {
	case len(body) > 0 && body[len(body)-1] == bel:
		body = body[:len(body)-1]
	case len(body) >= 2 && body[len(body)-2] == esc && body[len(body)-1] == '\\':
		body = body[:len(body)-2]
	}

	parts := strings.SplitN(string(body), ";", 2)
	var desc string
	switch parts[0] {
	case "0":
		desc = "set window title and icon name"
	case "1":
		desc = "set icon name"
	case "2":
		desc = "set window title"
	case "8":
		desc = "hyperlink"
	case "10":
		desc = "set default foreground color"
	case "11":
		desc = "set default background color"
	case "52":
		desc = "clipboard access"
	default:
		desc = "OSC command " + parts[0] + " (unrecognized)"
	}
	if len(parts) > 1 {
		desc += fmt.Sprintf(": %q", parts[1])
	}
	return Sequence{Pos: start, Raw: escapeVisual(string(raw)), Description: desc}
}

var simpleEscNames = map[byte]string{
	'7': "save cursor position and attributes (DEC)",
	'8': "restore cursor position and attributes (DEC)",
	'c': "full reset (RIS)",
	'D': "index (move down, scroll if needed)",
	'M': "reverse index (move up, scroll if needed)",
	'E': "next line",
	'=': "enable application keypad",
	'>': "disable application keypad",
}

var charsetSlots = map[byte]string{'(': "G0", ')': "G1", '*': "G2", '+': "G3"}

// scanSimple parses two-byte ESC sequences and the three-byte charset
// designation sequences (ESC ( X, ESC ) X, and so on).
func scanSimple(data []byte, start Pos) (Sequence, int, error) {
	b := data[1]

	if slot, ok := charsetSlots[b]; ok {
		if len(data) < 3 {
			return Sequence{}, 0, &ScanError{
				Pos: start,
				Msg: fmt.Sprintf("unterminated character set designation %s (reached end of input)", escapeVisual(string(data))),
			}
		}
		return Sequence{
			Pos:         start,
			Raw:         escapeVisual(string(data[:3])),
			Description: fmt.Sprintf("designate %s character set to %q", slot, string(data[2])),
		}, 3, nil
	}

	if name, ok := simpleEscNames[b]; ok {
		return Sequence{
			Pos:         start,
			Raw:         escapeVisual(string(data[:2])),
			Description: name,
		}, 2, nil
	}

	return Sequence{}, 0, &ScanError{
		Pos: start,
		Msg: fmt.Sprintf("unrecognized escape sequence %s (byte 0x%02X after ESC)", escapeVisual(string(data[:2])), b),
	}
}

// escapeVisual renders control bytes as readable names so raw sequences
// can be printed without corrupting the terminal running escq itself.
func escapeVisual(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case esc:
			b.WriteString("ESC")
		case bel:
			b.WriteString("BEL")
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
