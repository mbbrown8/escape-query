package main

import (
	"strings"
	"testing"
)

func TestScanCSI(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantDesc string
		wantN    int
	}{
		{"cursor up default", "\x1b[A", "cursor up 1", 3},
		{"cursor up with count", "\x1b[5A", "cursor up 5", 4},
		{"cursor position default", "\x1b[H", "cursor to row 1, column 1", 3},
		{"cursor position with args", "\x1b[10;20H", "cursor to row 10, column 20", 8},
		{"erase display default", "\x1b[J", "erase in display: from cursor to end", 3},
		{"sgr bold and red", "\x1b[1;31m", "bold; set foreground: red", 7},
		{"sgr 256 color", "\x1b[38;5;200m", "set foreground: 256-color palette index 200", 11},
		{"sgr truecolor", "\x1b[38;2;10;20;30m", "set foreground: rgb(10, 20, 30)", 16},
		{"private mode enable", "\x1b[?25h", "enable: cursor visibility", 6},
		{"private mode disable unrecognized", "\x1b[?9999l", "disable: 9999 (unrecognized)", 8},
		{"unrecognized final byte", "\x1b[Z", `CSI sequence, final byte "Z" (unrecognized)`, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, n, err := scanCSI([]byte(tt.in), Pos{Line: 1, Col: 1})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
			if seq.Description != tt.wantDesc {
				t.Errorf("description = %q, want %q", seq.Description, tt.wantDesc)
			}
		})
	}
}

func TestScanCSIErrors(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantContain string
	}{
		{"invalid byte in params", "\x1b[1\x00m", "invalid byte 0x00 in CSI sequence"},
		{"unterminated", "\x1b[31", "unterminated CSI sequence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := scanCSI([]byte(tt.in), Pos{Line: 1, Col: 1})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantContain)
			}
		})
	}
}

func TestScanOSC(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantDesc string
	}{
		{"window title with BEL", "\x1b]0;my title\x07", `set window title and icon name: "my title"`},
		{"hyperlink with ST", "\x1b]8;;http://example.com\x1b\\", `hyperlink: ";http://example.com"`},
		{"default background color", "\x1b]11;#000000\x07", `set default background color: "#000000"`},
		{"unrecognized command", "\x1b]999;foo\x07", `OSC command 999 (unrecognized): "foo"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, n, err := scanOSC([]byte(tt.in), Pos{Line: 1, Col: 1})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != len(tt.in) {
				t.Errorf("n = %d, want %d (full input consumed)", n, len(tt.in))
			}
			if seq.Description != tt.wantDesc {
				t.Errorf("description = %q, want %q", seq.Description, tt.wantDesc)
			}
		})
	}
}

func TestScanOSCErrors(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantContain string
	}{
		{"esc not followed by backslash", "\x1b]0;hi\x1bx", "ESC not followed by backslash"},
		{"reached end of input", "\x1b]0;hi", "expected BEL or ESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := scanOSC([]byte(tt.in), Pos{Line: 1, Col: 1})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantContain)
			}
		})
	}
}

func TestScanSimple(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantDesc string
		wantN    int
	}{
		{"save cursor DEC", "\x1b7", "save cursor position and attributes (DEC)", 2},
		{"full reset", "\x1bc", "full reset (RIS)", 2},
		{"next line", "\x1bE", "next line", 2},
		{"designate G0", "\x1b(B", `designate G0 character set to "B"`, 3},
		{"designate G1", "\x1b)0", `designate G1 character set to "0"`, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seq, n, err := scanSimple([]byte(tt.in), Pos{Line: 1, Col: 1})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
			if seq.Description != tt.wantDesc {
				t.Errorf("description = %q, want %q", seq.Description, tt.wantDesc)
			}
		})
	}
}

func TestScanSimpleErrors(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantContain string
	}{
		{"unterminated charset designation", "\x1b(", "unterminated character set designation"},
		{"unrecognized escape", "\x1bQ", "unrecognized escape sequence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := scanSimple([]byte(tt.in), Pos{Line: 1, Col: 1})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantContain) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantContain)
			}
		})
	}
}

func TestScanTracksLineAndColumn(t *testing.T) {
	data := []byte("hi\n\x1b[31mred\x1b[0m\n")

	seqs, errs := Scan(data)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(seqs) != 2 {
		t.Fatalf("got %d sequences, want 2", len(seqs))
	}

	if want := (Pos{Line: 2, Col: 1}); seqs[0].Pos != want {
		t.Errorf("seqs[0].Pos = %v, want %v", seqs[0].Pos, want)
	}
	if want := "set foreground: red"; seqs[0].Description != want {
		t.Errorf("seqs[0].Description = %q, want %q", seqs[0].Description, want)
	}

	if want := (Pos{Line: 2, Col: 9}); seqs[1].Pos != want {
		t.Errorf("seqs[1].Pos = %v, want %v", seqs[1].Pos, want)
	}
	if want := "reset all attributes"; seqs[1].Description != want {
		t.Errorf("seqs[1].Description = %q, want %q", seqs[1].Description, want)
	}
}

func TestScanLoneEscAtEndOfInput(t *testing.T) {
	seqs, errs := Scan([]byte("abc\x1b"))
	if len(seqs) != 0 {
		t.Fatalf("got %d sequences, want 0", len(seqs))
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errs))
	}

	scanErr, ok := errs[0].(*ScanError)
	if !ok {
		t.Fatalf("error type = %T, want *ScanError", errs[0])
	}
	if want := (Pos{Line: 1, Col: 4}); scanErr.Pos != want {
		t.Errorf("error pos = %v, want %v", scanErr.Pos, want)
	}
	if !strings.Contains(scanErr.Msg, "lone ESC at end of input") {
		t.Errorf("error message = %q, want it to contain %q", scanErr.Msg, "lone ESC at end of input")
	}
}
