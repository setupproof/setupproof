package markdown

import "strings"

const (
	MarkerFormHTMLComment = "html-comment"
	MarkerFormInfoString  = "info-string"
)

type Block struct {
	File       string
	Line       int
	MarkerLine int
	Language   string
	Shell      string
	Text       string
	MarkerForm string
	Metadata   map[string]string
	Warnings   []string
}

type Candidate struct {
	File     string
	Line     int
	Language string
	Shell    string
	Text     string
	Marked   bool
}

type fence struct {
	char  byte
	width int
	info  string
}

type marker struct {
	line     int
	form     string
	metadata map[string]string
	warnings []string
}

type fenceInfo struct {
	language string
	marker   *marker
}

func Discover(file string, contents []byte) []Block {
	lines := splitLines(string(contents))
	var blocks []Block
	var pending *marker

	for i := 0; i < len(lines); {
		if open, ok := openingFence(lines[i]); ok {
			info := parseFenceInfo(open.info, i+1)
			selected := info.marker
			if selected == nil && pending != nil {
				selected = pending
			}
			pending = nil

			contentStart := i + 1
			contentEnd := contentStart
			for contentEnd < len(lines) && !closingFence(lines[contentEnd], open) {
				contentEnd++
			}

			if selected != nil {
				if shell, ok := shellForLanguage(info.language); ok {
					blocks = append(blocks, Block{
						File:       file,
						Line:       i + 1,
						MarkerLine: selected.line,
						Language:   normalizeLanguage(info.language),
						Shell:      shell,
						Text:       strings.Join(lines[contentStart:contentEnd], ""),
						MarkerForm: selected.form,
						Metadata:   copyMetadata(selected.metadata),
						Warnings:   append([]string(nil), selected.warnings...),
					})
				}
			}

			if contentEnd < len(lines) {
				i = contentEnd + 1
			} else {
				i = len(lines)
			}
			continue
		}

		if next, ok := markerComment(lines[i], i+1); ok {
			pending = &next
			i++
			continue
		}

		if pending != nil && !blankLine(lines[i]) && !htmlCommentLine(lines[i]) {
			pending = nil
		}
		i++
	}

	return blocks
}

func Candidates(file string, contents []byte) []Candidate {
	lines := splitLines(string(contents))
	var candidates []Candidate
	var pending *marker

	for i := 0; i < len(lines); {
		if open, ok := openingFence(lines[i]); ok {
			info := parseFenceInfo(open.info, i+1)
			selected := info.marker
			if selected == nil && pending != nil {
				selected = pending
			}
			pending = nil

			contentStart := i + 1
			contentEnd := contentStart
			for contentEnd < len(lines) && !closingFence(lines[contentEnd], open) {
				contentEnd++
			}

			if shell, ok := shellForLanguage(info.language); ok {
				candidates = append(candidates, Candidate{
					File:     file,
					Line:     i + 1,
					Language: normalizeLanguage(info.language),
					Shell:    shell,
					Text:     strings.Join(lines[contentStart:contentEnd], ""),
					Marked:   selected != nil,
				})
			}

			if contentEnd < len(lines) {
				i = contentEnd + 1
			} else {
				i = len(lines)
			}
			continue
		}

		if next, ok := markerComment(lines[i], i+1); ok {
			pending = &next
			i++
			continue
		}

		if pending != nil && !blankLine(lines[i]) && !htmlCommentLine(lines[i]) {
			pending = nil
		}
		i++
	}

	return candidates
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.SplitAfter(text, "\n")
}

func trimLineEnding(line string) string {
	line = strings.TrimSuffix(line, "\n")
	return strings.TrimSuffix(line, "\r")
}

func openingFence(line string) (fence, bool) {
	trimmed := trimLineEnding(line)
	offset := 0
	for offset < len(trimmed) && trimmed[offset] == ' ' {
		offset++
	}
	if offset > 3 || offset >= len(trimmed) {
		return fence{}, false
	}

	char := trimmed[offset]
	if char != '`' && char != '~' {
		return fence{}, false
	}

	end := offset
	for end < len(trimmed) && trimmed[end] == char {
		end++
	}
	width := end - offset
	if width < 3 {
		return fence{}, false
	}

	return fence{
		char:  char,
		width: width,
		info:  strings.TrimSpace(trimmed[end:]),
	}, true
}

func closingFence(line string, open fence) bool {
	trimmed := trimLineEnding(line)
	offset := 0
	for offset < len(trimmed) && trimmed[offset] == ' ' {
		offset++
	}
	if offset > 3 || offset >= len(trimmed) || trimmed[offset] != open.char {
		return false
	}

	end := offset
	for end < len(trimmed) && trimmed[end] == open.char {
		end++
	}
	if end-offset < open.width {
		return false
	}

	return onlySpaceOrTab(trimmed[end:])
}

func parseFenceInfo(info string, line int) fenceInfo {
	tokens, tokenWarnings := markerTokens(info)
	if len(tokens) == 0 {
		return fenceInfo{}
	}

	result := fenceInfo{language: tokens[0]}
	if len(tokens) >= 2 && tokens[1] == "setupproof" {
		metadata, warnings := parseMetadata(tokens[2:])
		warnings = append(tokenWarnings, warnings...)
		result.marker = &marker{
			line:     line,
			form:     MarkerFormInfoString,
			metadata: metadata,
			warnings: warnings,
		}
	}
	return result
}

func markerComment(line string, number int) (marker, bool) {
	if !htmlCommentLine(line) {
		return marker{}, false
	}

	trimmed := strings.TrimSpace(trimLineEnding(line))
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
	tokens, tokenWarnings := markerTokens(inner)
	if len(tokens) == 0 || tokens[0] != "setupproof" {
		return marker{}, false
	}

	metadata, warnings := parseMetadata(tokens[1:])
	warnings = append(tokenWarnings, warnings...)
	return marker{
		line:     number,
		form:     MarkerFormHTMLComment,
		metadata: metadata,
		warnings: warnings,
	}, true
}

func parseMetadata(tokens []string) (map[string]string, []string) {
	metadata := make(map[string]string)
	var warnings []string
	for _, token := range tokens {
		key, value, ok := strings.Cut(token, "=")
		if !ok || key == "" {
			warnings = append(warnings, "marker metadata token "+token+" must use key=value")
			continue
		}
		metadata[key] = trimMatchingQuotes(value)
	}
	return metadata, warnings
}

func markerTokens(text string) ([]string, []string) {
	var tokens []string
	var warnings []string
	var current strings.Builder
	inToken := false
	var quote byte

	flush := func() {
		if !inToken {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
		inToken = false
	}

	for i := 0; i < len(text); i++ {
		ch := text[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				current.WriteByte(ch)
			}
			inToken = true
			continue
		}

		switch ch {
		case ' ', '\t', '\r', '\n':
			flush()
		case '\'', '"':
			quote = ch
			inToken = true
		default:
			current.WriteByte(ch)
			inToken = true
		}
	}
	if quote != 0 {
		warnings = append(warnings, "marker metadata has unterminated quote")
	}
	flush()
	return tokens, warnings
}

func trimMatchingQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	first := value[0]
	last := value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func shellForLanguage(language string) (string, bool) {
	switch normalizeLanguage(language) {
	case "sh":
		return "sh", true
	case "bash":
		return "bash", true
	case "shell":
		return "sh", true
	default:
		return "", false
	}
}

func normalizeLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func blankLine(line string) bool {
	return strings.TrimSpace(trimLineEnding(line)) == ""
}

func htmlCommentLine(line string) bool {
	trimmed := strings.TrimSpace(trimLineEnding(line))
	return strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->")
}

func onlySpaceOrTab(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] != ' ' && text[i] != '\t' {
			return false
		}
	}
	return true
}

func copyMetadata(metadata map[string]string) map[string]string {
	copied := make(map[string]string, len(metadata))
	for key, value := range metadata {
		copied[key] = value
	}
	return copied
}
