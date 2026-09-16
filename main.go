// Command escq scans text for terminal escape sequences and explains each
// one it finds, with a line and column number pointing at the ESC byte
// that started it.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "escq:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var r io.Reader = os.Stdin
	name := "<stdin>"
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		r = f
		name = args[0]
	}

	data, err := io.ReadAll(bufio.NewReader(r))
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}

	seqs, errs := Scan(data)

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	for _, s := range seqs {
		fmt.Fprintf(w, "%-10s %-22s %s\n", s.Pos, s.Raw, s.Description)
	}

	if len(errs) > 0 {
		w.Flush()
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "%s: %s\n", name, e)
		}
		return fmt.Errorf("%d error(s) found in %s", len(errs), name)
	}

	return nil
}
