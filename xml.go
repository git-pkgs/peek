package peek

import "bytes"

func (s *scanner) xml(start int) {
	const prefix = "<?xml"
	data := s.data[start:]
	if !bytes.HasPrefix(data, []byte(prefix)) || len(data) <= len(prefix) || !xmlSpace(data[len(prefix)]) {
		return
	}
	end := bytes.Index(data, []byte("?>"))
	if end < 0 {
		return
	}
	span := Span{start, start + end + len("?>")}
	s.emit(XMLDeclaration, "xml-prolog", span, span, "")
	s.xmlEncoding(span)
}

func (s *scanner) xmlEncoding(declaration Span) {
	data := s.data
	for pos := declaration.Start + len("<?xml"); pos < declaration.End-2; {
		if !xmlSpace(data[pos]) {
			return
		}
		for pos < declaration.End && xmlSpace(data[pos]) {
			pos++
		}
		start := pos
		for pos < declaration.End && encodingByte(data[pos]) {
			pos++
		}
		name := data[start:pos]
		for pos < declaration.End && xmlSpace(data[pos]) {
			pos++
		}
		if pos >= declaration.End || data[pos] != '=' {
			return
		}
		pos++
		for pos < declaration.End && xmlSpace(data[pos]) {
			pos++
		}
		if pos >= declaration.End || (data[pos] != '\'' && data[pos] != '"') {
			return
		}
		quote := data[pos]
		pos++
		start = pos
		for pos < declaration.End && data[pos] != quote {
			pos++
		}
		if pos == declaration.End {
			return
		}
		if bytes.Equal(name, []byte("encoding")) && pos > start {
			s.emit(Encoding, "xml-encoding", declaration, Span{start, pos}, "")
		}
		pos++
	}
}

func xmlSpace(c byte) bool { return c == '\n' || space(c) }
