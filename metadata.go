package peek

import "bytes"

// header selects metadata candidates after a conventional comment introducer.
func (s *scanner) header(body Span, terminated bool) (Span, bool) {
	data := s.data[body.Start:body.End]
	if len(data) == 0 {
		return body, terminated
	}
	switch data[0] {
	case '#', ';', '*':
		body.Start++
	case '/':
		if bytes.HasPrefix(data, []byte("//")) || bytes.HasPrefix(data, []byte("/*")) {
			body.Start += len("//")
		}
	case '-':
		if bytes.HasPrefix(data, []byte("--")) {
			body.Start += len("--")
		}
	case '<':
		if bytes.HasPrefix(data, []byte("<!--")) {
			body.Start += len("<!--")
		}
	}
	body = s.trim(body)
	if body.Start == body.End {
		return body, terminated
	}
	switch s.data[body.Start] {
	case 'S', 'C', 'c', 's', 'G', 'g', 'p':
	default:
		return Span{body.Start, body.Start}, terminated
	}
	suffix := ""
	switch {
	case data[0] == '*' || bytes.HasPrefix(data, []byte("/*")):
		suffix = "*/"
	case bytes.HasPrefix(data, []byte("<!--")):
		suffix = "-->"
	}
	if suffix != "" {
		if end := bytes.Index(s.data[body.Start:body.End], []byte(suffix)); end >= 0 {
			body.End = body.Start + end
			terminated = true
		}
	}
	return s.trim(body), terminated
}

func (s *scanner) metadata(line Span, terminated bool) {
	trimmed := s.trim(line)
	body, ended := s.header(trimmed, terminated)
	if !ended {
		return
	}
	data := s.data[trimmed.Start:trimmed.End]
	if bytes.HasPrefix(data, []byte("//go:build ")) ||
		bytes.HasPrefix(data, []byte("//go:generate ")) ||
		bytes.HasPrefix(data, []byte("#pragma ")) {
		s.field(Directive, "directive-line", line, trimmed)
	}
	if body.Start == body.End {
		return
	}
	for _, rule := range [...]struct {
		prefix string
		kind   Kind
		name   string
	}{
		{"SPDX-License-Identifier:", SPDXLicense, "spdx-license-tag"},
		{"SPDX-FileCopyrightText:", Copyright, "spdx-copyright-tag"},
		{"source:", Source, "source-comment"},
		{"Generated from ", Source, "generated-from-comment"},
	} {
		if bytes.HasPrefix(s.data[body.Start:body.End], []byte(rule.prefix)) {
			s.field(rule.kind, rule.name, line, Span{body.Start + len(rule.prefix), body.End})
		}
	}
	s.copyright(line, body)
	s.generated(line, body)
}

func (s *scanner) field(kind Kind, rule string, evidence, value Span) {
	value = s.trim(value)
	if value.Start < value.End {
		s.emit(kind, rule, evidence, value, "")
	}
}

func (s *scanner) copyright(line, body Span) {
	const word = "copyright"
	data := s.data[body.Start:body.End]
	if len(data) > len(word) && bytes.EqualFold(data[:len(word)], []byte(word)) && space(data[len(word)]) {
		s.emit(Copyright, "copyright-notice", line, body, "")
	}
}

func (s *scanner) generated(line, body Span) {
	const prefix = "Code generated "
	const suffix = ". DO NOT EDIT."
	data := s.data[body.Start:body.End]
	if bytes.HasPrefix(data, []byte(prefix)) && bytes.HasSuffix(data, []byte(suffix)) && len(data) > len(prefix)+len(suffix) {
		s.field(Generated, "go-generated-marker", line, Span{body.Start + len(prefix), body.End - len(suffix)})
	}
}

func (s *scanner) coding(line Span, terminated bool) {
	body := s.trim(line)
	if body.Start == body.End || s.data[body.Start] != '#' {
		return
	}
	data := s.data[body.Start:body.End]
	index := bytes.Index(data, []byte("coding"))
	if index < 0 {
		return
	}
	start := body.Start + index + len("coding")
	if start == body.End || (s.data[start] != ':' && s.data[start] != '=') {
		return
	}
	value := s.trim(Span{start + 1, body.End})
	end := value.Start
	for end < value.End && encodingByte(s.data[end]) {
		end++
	}
	if end < line.End || terminated {
		s.field(Encoding, "coding-cookie", line, Span{value.Start, end})
	}
}

func encodingByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.'
}
