package runner

import (
	"regexp"
	"strings"
)

var interactivePatterns = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{name: "read", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])read([[:space:]]|$)`)},
	{name: "select", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])select([[:space:]]|$)`)},
	{name: "passwd", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])passwd([[:space:]]|$)`)},
	{name: "ssh-keygen", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])ssh-keygen([[:space:]]|$)`)},
	{name: "npm login", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])npm[[:space:]]+login([[:space:]]|$)`)},
	{name: "docker login", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])docker[[:space:]]+login([[:space:]]|$)`)},
	{name: "gh auth login", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])gh[[:space:]]+auth[[:space:]]+login([[:space:]]|$)`)},
	{name: "aws configure", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])aws[[:space:]]+configure([[:space:]]|$)`)},
	{name: "gcloud auth login", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])gcloud[[:space:]]+auth[[:space:]]+login([[:space:]]|$)`)},
	{name: "az login", pattern: regexp.MustCompile(`(^|[;&|({[:space:]])az[[:space:]]+login([[:space:]]|$)`)},
}

func classifyInteractive(source string) (string, bool) {
	for _, line := range strings.Split(joinShellContinuations(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, candidate := range interactivePatterns {
			if candidate.pattern.MatchString(trimmed) {
				return candidate.name, true
			}
		}
	}
	return "", false
}

func joinShellContinuations(source string) string {
	source = strings.ReplaceAll(source, "\\\r\n", "")
	source = strings.ReplaceAll(source, "\\\n", "")
	return source
}
