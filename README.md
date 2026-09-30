# peek

peek extracts explicit claims from files and bounded file prefixes, including
the roughly 1 KB prefixes in Software Heritage exports. The library is pure
Go, uses only the standard library, and performs no network or filesystem I/O.
It needs no CGO or package initialization work.

Absence from a prefix is not evidence of absence from the complete file.
An empty result means no supported claims were found in the supplied window.
Even complete input can contain metadata outside the supported conventions.

## Install

The Go module is `github.com/git-pkgs/peek`. Build the command from this checkout:

```bash
CGO_ENABLED=0 go build -o /tmp/peek ./cmd/peek
```

## Use

```go
data := []byte("// SPDX-License-Identifier: MIT\npackage exam")
result := peek.Inspect(peek.Input{Bytes: data})
for _, claim := range result.Claims {
    fmt.Printf("%s: %s\n", claim.Kind, data[claim.Value.Start:claim.Value.End])
}
```

`Input.Bytes` must start at file offset zero. Set `Complete: true` only when
the slice contains the complete file; otherwise the final line may continue
beyond the supplied bytes. For example, `SPDX-License-Identifier: MIT` could
continue with ` OR Apache-2.0`. That tag produces a licence claim only after
a line or comment ending, or when the caller confirms EOF.

Use `InspectInto(&result, input)` to reuse claim storage between calls, with
one result per concurrent worker. Both APIs leave the input in place and
retain only offsets into it. Consume claim values before reusing the input
buffer. The library has no mutable package state.

`Inspect` returns independent claim storage. `InspectInto` requires a non-nil
result and may overwrite its previous claims; copy the claim slice if you need
to retain an earlier result. Copying a `Result` alone shares that slice. An empty
claim slice may be nil.

The command writes one JSON record per file, with claim text and byte ranges:

```bash
/tmp/peek path/to/source.go
/tmp/peek -bytes 4096 path/to/source.go other.py
/tmp/peek -prefix < testdata/python.prefix
```

With no paths, or a path of `-`, the command reads stdin. It defaults to 1024
bytes and reads at most one extra byte to check whether the file continues.
Use `-prefix` for an already truncated export or stream: its EOF does not
establish the original file's EOF. A reader error fails the command. Invalid
UTF-8 claim values include `raw_base64` to preserve their bytes in JSON.

Each JSON record has `path`, `input_bytes`, `input_complete`, `scanned_bytes`,
`bytes_limited`, `claims_limited`, and `claims` fields. `claims` is an array,
including when empty. Each entry contains `kind`, `rule`, `evidence`, `value`,
and `text`; `label` and `raw_base64` appear only when applicable. Span objects
contain `start` and `end` offsets. Consumers should allow additional JSON fields.

## Claims

Each `Claim` contains a `Kind`, a named `Rule`, an `Evidence` span and a `Value`
span. Spans are zero-based, half-open byte offsets into the original input;
the value is inside the evidence. `Label` identifies a BOM encoding. Other
values remain exactly as written, without normalizing case or licence names.

| Kind | Rule | Supported evidence |
| --- | --- | --- |
| `bom` | `byte-order-mark` | UTF-8, UTF-16 and UTF-32 byte-order marks |
| `shebang`, `interpreter`, `interpreter-arguments` | `shebang-line`, `shebang-path`, `shebang-arguments` | `#!` at byte zero; executable path and unsplit argument text |
| `encoding-declaration` | `coding-cookie`, `xml-encoding` | `coding:` or `coding=` in a hash comment on the first two lines; quoted XML prolog encoding |
| `spdx-license` | `spdx-license-tag` | A leading `SPDX-License-Identifier:` tag after an optional comment introducer |
| `copyright` | `spdx-copyright-tag`, `copyright-notice` | `SPDX-FileCopyrightText:` or a leading case-insensitive `Copyright` notice |
| `generated-marker` | `go-generated-marker` | `Code generated <text> DO NOT EDIT.` header convention |
| `directive` | `directive-line` | `//go:build`, `//go:generate`, and `#pragma` lines |
| `source-reference` | `source-comment`, `generated-from-comment` | Leading `source:` and `Generated from` header text |
| `xml-declaration` | `xml-prolog` | A closed `<?xml ... ?>` prolog at the start, optionally after a UTF-8 BOM |
| `url` | `http-token` | Delimited lowercase `http://` and `https://` tokens |

`Kind` identifies the observation type, and `Rule` identifies its matching
convention. Consumers should allow new kinds and rules. Claim order has no API
guarantee; sort by spans when source order is needed. As a prefix grows, a
claim's evidence span may expand while its value span stays fixed. Repeated
observations at different offsets remain separate claims.

A matching header produces a claim even inside a multiline string or an
example. An SPDX claim records the observed tag without validating its
expression or establishing the file's applicable licence. REUSE ignore
blocks and snippet scopes are not interpreted. Generator names and versions
remain together in the extracted text, including punctuation before ` DO NOT EDIT.`.

Shebang arguments stay raw because kernel and `env` argument handling varies.
For `#!/usr/bin/env -S python3 -u`, the interpreter claim is `/usr/bin/env` and
the arguments are `-S python3 -u`, with no environment lookup or execution.
A delimited interpreter path can produce a claim before the rest of its line
is available; whole-line claims require a terminator or confirmed EOF.

Text extraction handles ASCII-compatible byte layouts without transcoding
and stops after reporting a wide-character BOM. Ambiguous short
UTF-16LE/UTF-32LE BOM prefixes produce no BOM claim. URL claims retain
punctuation without validation or resolution. XML processing does not expand
entities or load schemas.

Package and namespace declarations, XML doctypes and namespace binding,
standard identifiers, and additional generated-code conventions are outside
the current extractor set. Imports, dependency analysis, language inference,
ASTs, and semantic analysis belong to other tools.

## Composition

| Project | Question |
| --- | --- |
| `magic` | What kind of content is this? |
| `peek` | What explicit facts can we observe from this bounded prefix? |
| `languages` | What programming language does this appear to be? |
| `outline` | What is the structural shape of the complete source? |
| `brief` | What higher-level classification can we infer? |

Call `magic.DetectPrefix(data)` alongside `peek.Inspect` when content
identification is useful; use `magic.Detect` only for complete content. Peek's
line-based extraction stops at certain control bytes and resumes on the next
line. It has no binary signatures or text/binary classification.

Use `archives` to expose member bytes and pass a bounded member prefix to
`peek`. Keep archive input, expansion, member-count and nesting limits in the
caller. Mark a member complete only when its end is established; ending a
limited reader is insufficient. Peek does not open or decompress archives.

Pass extracted SPDX values to `spdx` when syntax or identifier validation is
needed. Use `licenses` for licence-text matching, or `reuse` for project
headers, sidecars and annotations. Preserve the original claim and input
completeness alongside any conclusions from those tools.

## Performance

Each call inspects a window of at most 64 KiB and emits at most 256 claims.
`InputBytes` and `InputComplete` describe the supplied input. `ScannedBytes`
is the bounded window size. `BytesLimited` reports clipping of supplied bytes;
`ClaimsLimited` reports omitted claims after reaching the output cap. These
flags describe limits reached during extraction, independent of input
completeness.

Work is linear in the bounded window, with a fixed set of byte scans and no
regex engine, recursion, parser tables or global cache. Reused results allocate
no memory for the benchmark fixtures. Consuming each result before the next
input keeps memory independent of corpus size; retaining results requires
additional storage in the caller.

```bash
make test
make bench
make profile
```

Benchmarks cover 1 KB inputs, fresh and reused results, a 4096-prefix mixed
batch, parallel workers, adversarial inputs, and batch heap usage. Go reports
ns/op, B/op, allocs/op and MB/s. For a million-item memory run:

```bash
go test -c -o /tmp/peek-bench.test .
/tmp/peek-bench.test -test.run '^$' -test.bench '^BenchmarkBatchMemory$' -test.benchtime 1000000x -test.benchmem
```

Tests include synthetic source-style fixtures, every byte cut of each fixture,
CLI reads at exact size boundaries, output limits, reuse, concurrency, and
fuzzing. CI builds and tests with `CGO_ENABLED=0`, runs the race detector and
builds WASI without native dependencies.

## License

[MIT](LICENSE).
