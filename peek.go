// Package peek extracts evidence-backed claims from bounded file content.
// Missing claims never establish absence, even for complete input.
package peek

const (
	MaxBytes  = 64 * 1024
	MaxClaims = 256
)

// Input starts at file offset zero. Complete is false unless EOF is known.
type Input struct {
	Bytes    []byte
	Complete bool
}

// Span is a half-open byte range in Input.Bytes, without decoding or copying.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Kind identifies the type of lexical evidence recorded by a claim.
type Kind string

const (
	BOM                  Kind = "bom"
	Shebang              Kind = "shebang"
	Interpreter          Kind = "interpreter"
	InterpreterArguments Kind = "interpreter-arguments"
	Encoding             Kind = "encoding-declaration"
	SPDXLicense          Kind = "spdx-license"
	Copyright            Kind = "copyright"
	Generated            Kind = "generated-marker"
	Directive            Kind = "directive"
	Source               Kind = "source-reference"
	URL                  Kind = "url"
	XMLDeclaration       Kind = "xml-declaration"
)

// Claim records lexical evidence, not a verified property of the file.
// Value lies inside Evidence. Label names a BOM encoding; other values stay raw.
// Rule identifies the matching convention. Evidence may grow as input is extended.
type Claim struct {
	Kind     Kind   `json:"kind"`
	Rule     string `json:"rule"`
	Evidence Span   `json:"evidence"`
	Value    Span   `json:"value"`
	Label    string `json:"label,omitempty"`
}

// Result owns only its claim slice, never the input bytes.
// InputComplete describes the caller's input, independently of resource limits.
// BytesLimited and ClaimsLimited mean extraction omitted available data.
// Claims may be nil when empty. Their order is not an API guarantee.
// Copying a Result shares its claim storage.
type Result struct {
	InputBytes    int     `json:"input_bytes"`
	InputComplete bool    `json:"input_complete"`
	ScannedBytes  int     `json:"scanned_bytes"`
	BytesLimited  bool    `json:"bytes_limited"`
	ClaimsLimited bool    `json:"claims_limited"`
	Claims        []Claim `json:"claims"`
}

// Inspect extracts at most MaxClaims claims without modifying or retaining
// input.Bytes. Each result has independent claim storage.
func Inspect(input Input) Result {
	var result Result
	InspectInto(&result, input)
	return result
}

// InspectInto replaces a non-nil result and reuses its claim storage, which may
// overwrite earlier claims. Copy claims to retain them across calls. Independent
// results may be used concurrently; do not mutate input.Bytes during a call.
func InspectInto(result *Result, input Input) {
	*result = Result{
		InputBytes: len(input.Bytes), InputComplete: input.Complete,
		ScannedBytes: min(len(input.Bytes), MaxBytes),
		BytesLimited: len(input.Bytes) > MaxBytes,
		Claims:       result.Claims[:0],
	}
	data := input.Bytes[:result.ScannedBytes]
	s := scanner{data: data, result: result, complete: input.Complete && !result.BytesLimited}
	start, wide := s.bom()
	if !wide {
		s.xml(start)
		s.lines(start)
	}
}

type scanner struct {
	data     []byte
	result   *Result
	complete bool
}

func (s *scanner) emit(kind Kind, rule string, evidence, value Span, label string) {
	if len(s.result.Claims) == MaxClaims {
		s.result.ClaimsLimited = true
		return
	}
	s.result.Claims = append(s.result.Claims, Claim{
		Kind: kind, Rule: rule, Evidence: evidence, Value: value, Label: label,
	})
}
