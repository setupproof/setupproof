package adoption

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/setupproof/setupproof/internal/markdown"
	"github.com/setupproof/setupproof/internal/planning"
)

type Suggestion struct {
	File            string
	Line            int
	Language        string
	Confidence      string
	Reason          string
	RiskFlags       []string
	SuggestedMarker string
	NextCommand     string
}

func Suggest(req planning.Request) ([]Suggestion, error) {
	targets, err := planning.ResolveTargets(req)
	if err != nil {
		return nil, err
	}

	var suggestions []Suggestion
	for _, target := range targets {
		contents, err := os.ReadFile(target.Abs)
		if err != nil {
			return nil, err
		}
		for _, candidate := range markdown.Candidates(target.Rel, contents) {
			if candidate.Marked {
				continue
			}
			suggestions = append(suggestions, suggestionFor(candidate))
		}
	}
	return suggestions, nil
}

func suggestionFor(candidate markdown.Candidate) Suggestion {
	risks := riskFlags(candidate.Text)
	confidence := "medium"
	reason := "contains shell commands"
	if len(risks) == 0 {
		confidence = "high"
		reason = "looks like a setup or verification command"
	}
	if contains(risks, "pipes-remote-script") || contains(risks, "destructive") || contains(risks, "requires-secrets") {
		confidence = "low"
		reason = "contains high-risk command patterns"
	}

	return Suggestion{
		File:            candidate.File,
		Line:            candidate.Line,
		Language:        candidate.Language,
		Confidence:      confidence,
		Reason:          reason,
		RiskFlags:       risks,
		SuggestedMarker: fmt.Sprintf("<!-- setupproof id=%s -->", suggestedID(candidate)),
		NextCommand:     strings.TrimSpace(firstNonBlankLine(candidate.Text)),
	}
}

func riskFlags(text string) []string {
	lower := strings.ToLower(text)
	tokens := shellishTokens(lower)
	seen := make(map[string]bool)
	add := func(flag string) {
		seen[flag] = true
	}

	if hasToken(tokens, "curl") || hasToken(tokens, "wget") || strings.Contains(lower, "npm install") ||
		strings.Contains(lower, "pip install") || strings.Contains(lower, "go install") || strings.Contains(lower, "cargo install") ||
		strings.Contains(lower, "git clone") {
		add("network")
	}
	if hasToken(tokens, "curl") && (strings.Contains(lower, "| sh") || strings.Contains(lower, "| bash")) {
		add("pipes-remote-script")
		add("network")
	}
	if hasToken(tokens, "wget") && (strings.Contains(lower, "| sh") || strings.Contains(lower, "| bash")) {
		add("pipes-remote-script")
		add("network")
	}
	for _, pattern := range []string{"rm -rf", "mkfs", "dd if=", "chmod -r", "chown -r"} {
		if strings.Contains(lower, pattern) {
			add("destructive")
		}
	}
	for _, pattern := range []string{"sudo ", " sudo"} {
		if strings.Contains(lower, pattern) {
			add("uses-sudo")
		}
	}
	for _, pattern := range []string{"read", "select", "ssh-keygen", "passwd", "login"} {
		if hasToken(tokens, pattern) {
			add("interactive")
		}
	}
	for _, pattern := range []string{"npm run dev", "yarn dev", "pnpm dev", "vite", "next dev", "rails server", "python -m http.server", "serve", "docker compose up"} {
		if strings.Contains(lower, pattern) {
			add("starts-service")
			add("long-running")
		}
	}
	for _, pattern := range []string{"watch ", "tail -f", "sleep infinity"} {
		if strings.Contains(lower, pattern) {
			add("long-running")
		}
	}
	for _, pattern := range []string{"api_key", "apikey", "token", "secret", "password"} {
		if hasToken(tokens, pattern) {
			add("requires-secrets")
		}
	}
	for _, token := range tokens {
		if strings.HasPrefix(token, "aws_") || strings.HasPrefix(token, "gcp_") || strings.HasPrefix(token, "azure_") {
			add("requires-secrets")
		}
	}

	flags := make([]string, 0, len(seen))
	for flag := range seen {
		flags = append(flags, flag)
	}
	sort.Strings(flags)
	return flags
}

func shellishTokens(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == '/' || r == ':' ||
			(r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.Trim(field, "\"'")
		if trimmed == "" {
			continue
		}
		tokens = append(tokens, trimmed)
		if base := pathBase(trimmed); base != "" && base != trimmed {
			tokens = append(tokens, base)
		}
	}
	return tokens
}

func pathBase(value string) string {
	value = strings.TrimRight(value, "/")
	index := strings.LastIndex(value, "/")
	if index < 0 || index == len(value)-1 {
		return ""
	}
	return value[index+1:]
}

func hasToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func suggestedID(candidate markdown.Candidate) string {
	command := strings.ToLower(firstNonBlankLine(candidate.Text))
	switch {
	case strings.Contains(command, "install"):
		return "install"
	case strings.Contains(command, "test"):
		return "test"
	case strings.Contains(command, "build"):
		return "build"
	case strings.Contains(command, "start"), strings.Contains(command, "dev"):
		return "start"
	default:
		return fmt.Sprintf("line-%d", candidate.Line)
	}
}

func firstNonBlankLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
