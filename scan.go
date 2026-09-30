package peek

import "bytes"

const codingLines = 2

func (s *scanner) bom() (int, bool) {
	data := s.data
	label, size := "", 0
	switch {
	case bytes.HasPrefix(data, []byte{0, 0, 0xfe, 0xff}):
		label, size = "utf-32be", 4
	case bytes.HasPrefix(data, []byte{0xff, 0xfe, 0, 0}):
		label, size = "utf-32le", 4
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		label, size = "utf-8", 3
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		label, size = "utf-16be", 2
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		// A short little-endian prefix may still be a UTF-32 BOM.
		if !s.complete && len(data) < 4 && (len(data) == 2 || data[2] == 0) {
			return 0, true
		}
		label, size = "utf-16le", 2
	}
	if size > 0 {
		span := Span{0, size}
		s.emit(BOM, "byte-order-mark", span, span, label)
	}
	return size, size > 0 && label != "utf-8"
}

func (s *scanner) lines(start int) {
	for number := 1; start < len(s.data) && !s.result.ClaimsLimited; number++ {
		end := len(s.data)
		terminated := s.complete
		if next := bytes.IndexByte(s.data[start:], '\n'); next >= 0 {
			end, terminated = start+next, true
		}
		line := Span{start, end}
		if end > start && s.data[end-1] == '\r' && terminated {
			line.End--
		}
		if control := bytes.IndexAny(s.data[line.Start:line.End], "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0e\x0f"); control >= 0 {
			line.End, terminated = line.Start+control, false
		}
		s.line(line, number, terminated)
		start = end + 1
	}
}

func (s *scanner) line(line Span, number int, terminated bool) {
	if number == 1 && line.Start == 0 {
		s.shebang(line, terminated)
	}
	if number <= codingLines {
		s.coding(line, terminated)
	}
	s.metadata(line, terminated)
	s.urls(line, terminated)
}

func (s *scanner) shebang(line Span, terminated bool) {
	if !bytes.HasPrefix(s.data[line.Start:line.End], []byte("#!")) {
		return
	}
	value := s.trim(Span{line.Start + 2, line.End})
	if value.Start == value.End {
		return
	}
	end := value.Start
	for end < value.End && !space(s.data[end]) {
		end++
	}
	if end < line.End || terminated {
		s.emit(Interpreter, "shebang-path", Span{line.Start, end}, Span{value.Start, end}, "")
	}
	if !terminated {
		return
	}
	s.emit(Shebang, "shebang-line", line, value, "")
	args := s.trim(Span{end, value.End})
	if args.Start < args.End {
		s.emit(InterpreterArguments, "shebang-arguments", line, args, "")
	}
}

func (s *scanner) trim(span Span) Span {
	for span.Start < span.End && space(s.data[span.Start]) {
		span.Start++
	}
	for span.End > span.Start && space(s.data[span.End-1]) {
		span.End--
	}
	return span
}

func space(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\f' }
