package peek

import "bytes"

func (s *scanner) urls(line Span, terminated bool) {
	for offset := line.Start; offset < line.End && !s.result.ClaimsLimited; {
		index := bytes.Index(s.data[offset:line.End], []byte("http"))
		if index < 0 {
			return
		}
		start := offset + index
		offset = start + len("http")
		if start > line.Start && encodingByte(s.data[start-1]) {
			continue
		}
		scheme := 0
		switch {
		case bytes.HasPrefix(s.data[start:line.End], []byte("https://")):
			scheme = len("https://")
		case bytes.HasPrefix(s.data[start:line.End], []byte("http://")):
			scheme = len("http://")
		default:
			continue
		}
		end := start + scheme
		for end < line.End && !urlEnd(s.data[end]) {
			if s.data[end] == '*' && end+1 < line.End && s.data[end+1] == '/' {
				break
			}
			end++
		}
		if end > start+scheme && (end < line.End || terminated) {
			span := Span{start, end}
			s.emit(URL, "http-token", span, span, "")
		}
		offset = end
	}
}

func urlEnd(c byte) bool {
	return c <= ' ' || c == 0x7f || c == '"' || c == '\'' || c == '<' || c == '>' || c == '`'
}
