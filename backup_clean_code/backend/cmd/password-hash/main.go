package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"incidenthub/backend/internal/security"
)

func main() {
	if err := run(os.Stdin, os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(stdin io.Reader, stdout io.Writer, args []string) error {
	fs := flag.NewFlagSet("password-hash", flag.ContinueOnError)
	passwordFlag := fs.String("password", "", "plain password to hash")
	readFromStdin := fs.Bool("stdin", false, "read password from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}

	password := strings.TrimSpace(*passwordFlag)
	if password == "" && fs.NArg() > 0 {
		password = strings.TrimSpace(fs.Arg(0))
	}

	if password == "" && *readFromStdin {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		password = strings.TrimSpace(string(raw))
	}

	if password == "" {
		return fmt.Errorf("password is required (use -password, positional arg, or -stdin)")
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, hash); err != nil {
		return fmt.Errorf("write hash: %w", err)
	}
	return nil
}
