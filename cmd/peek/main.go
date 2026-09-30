package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"github.com/git-pkgs/peek"
)

const defaultBytes = 1024

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("peek", flag.ContinueOnError)
	flags.SetOutput(stderr)
	limit := flags.Int("bytes", defaultBytes, "maximum bytes to inspect (1..65536)")
	prefix := flags.Bool("prefix", false, "input is already a prefix; EOF does not mean a complete file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *limit < 1 || *limit > peek.MaxBytes {
		return fmt.Errorf("bytes must be between 1 and %d", peek.MaxBytes)
	}
	paths := flags.Args()
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	buffer := make([]byte, *limit+1)
	var result peek.Result
	encoder := json.NewEncoder(stdout)
	for _, path := range paths {
		if err := inspectPath(path, stdin, encoder, buffer, *prefix, &result); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func inspectPath(path string, stdin io.Reader, encoder *json.Encoder, buffer []byte, prefix bool, result *peek.Result) (resultErr error) {
	reader := stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
		reader = file
	}
	count, err := io.ReadFull(reader, buffer)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	limit := len(buffer) - 1
	input := peek.Input{Bytes: buffer[:min(count, limit)], Complete: count <= limit && !prefix}
	peek.InspectInto(result, input)
	return encoder.Encode(record(path, input.Bytes, result))
}

type claimJSON struct {
	peek.Claim
	Text string `json:"text"`
	Raw  []byte `json:"raw_base64,omitempty"`
}

type outputJSON struct {
	Path          string      `json:"path"`
	InputBytes    int         `json:"input_bytes"`
	InputComplete bool        `json:"input_complete"`
	ScannedBytes  int         `json:"scanned_bytes"`
	BytesLimited  bool        `json:"bytes_limited"`
	ClaimsLimited bool        `json:"claims_limited"`
	Claims        []claimJSON `json:"claims"`
}

func record(path string, data []byte, result *peek.Result) outputJSON {
	output := outputJSON{
		Path: path, InputBytes: result.InputBytes, InputComplete: result.InputComplete,
		ScannedBytes: result.ScannedBytes, BytesLimited: result.BytesLimited, ClaimsLimited: result.ClaimsLimited,
		Claims: make([]claimJSON, 0, len(result.Claims)),
	}
	for _, claim := range result.Claims {
		value := data[claim.Value.Start:claim.Value.End]
		entry := claimJSON{Claim: claim, Text: string(value)}
		if !utf8.Valid(value) {
			entry.Raw = value
		}
		output.Claims = append(output.Claims, entry)
	}
	return output
}
