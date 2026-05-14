package integrationpkg

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

type LintSeverity string

const (
	LintError   LintSeverity = "error"
	LintWarning LintSeverity = "warn"
)

type LintIssue struct {
	Integration string       `json:"integration"`
	Field       string       `json:"field"`
	Severity    LintSeverity `json:"severity"`
	Message     string       `json:"message"`
}

func (i LintIssue) Error() string {
	if i.Field == "" {
		return fmt.Sprintf("%s: %s", i.Integration, i.Message)
	}
	return fmt.Sprintf("%s: %s: %s", i.Integration, i.Field, i.Message)
}

func (i LintIssue) IsError() bool {
	return i.Severity == LintError
}

func HasLintErrors(issues []LintIssue) bool {
	for _, issue := range issues {
		if issue.IsError() {
			return true
		}
	}
	return false
}

func Lint(integ Integration) []LintIssue {
	l := integrationLinter{integration: integ}
	l.required("name", integ.Name)
	l.required("version", integ.Version)
	l.description("description", integ.Description, 20, 100)
	l.name("name", integ.Name)
	l.receiver()
	l.auth()
	l.configSchema()
	l.tools()
	l.docsURL()
	return l.issues
}

type integrationLinter struct {
	integration Integration
	issues      []LintIssue
}

var (
	integrationNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	toolNameRE        = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	fieldNameRE       = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

func (l *integrationLinter) add(field string, severity LintSeverity, message string) {
	l.issues = append(l.issues, LintIssue{
		Integration: l.integration.Name,
		Field:       field,
		Severity:    severity,
		Message:     message,
	})
}

func (l *integrationLinter) required(field, value string) {
	if strings.TrimSpace(value) == "" {
		l.add(field, LintError, "is required")
	}
}

func (l *integrationLinter) description(field, value string, minLen, maxLen int) {
	value = strings.TrimSpace(value)
	if value == "" {
		l.add(field, LintError, "is required")
		return
	}
	if len(value) < minLen || len(value) > maxLen {
		l.add(field, LintError, fmt.Sprintf("must be %d-%d characters", minLen, maxLen))
	}
	for _, phrase := range []string{"when you need data", "for general purposes", "anytime", "always"} {
		if strings.Contains(strings.ToLower(value), phrase) {
			l.add(field, LintError, "contains lazy phrase: "+phrase)
		}
	}
}

func (l *integrationLinter) name(field, value string) {
	if value == "" {
		return
	}
	if !integrationNameRE.MatchString(value) {
		l.add(field, LintError, "must be lowercase kebab-case")
	}
}

func (l *integrationLinter) receiver() {
	switch l.integration.Receiver.Type {
	case "none":
		if l.integration.Receiver.Path != "" {
			l.add("receiver.path", LintWarning, "is ignored when receiver.type is none")
		}
	case "webhook":
		if !strings.HasPrefix(l.integration.Receiver.Path, "/") {
			l.add("receiver.path", LintError, "must start with / for webhook receivers")
		}
	case "":
		l.add("receiver.type", LintError, "is required")
	default:
		l.add("receiver.type", LintError, "must be webhook or none")
	}
}

func (l *integrationLinter) auth() {
	switch l.integration.Auth.Type {
	case "none", "api_key", "oauth", "basic", "bearer_token", "kubeconfig", "aws_credentials":
	case "":
		l.add("auth.type", LintError, "is required")
	default:
		l.add("auth.type", LintError, "is not supported")
	}
	for idx, field := range l.integration.Auth.Fields {
		fieldPath := fmt.Sprintf("auth.fields[%d].name", idx)
		if strings.TrimSpace(field.Name) == "" {
			l.add(fieldPath, LintError, "is required")
			continue
		}
		if !fieldNameRE.MatchString(field.Name) {
			l.add(fieldPath, LintError, "must be snake_case")
		}
	}
}

func (l *integrationLinter) configSchema() {
	for key, field := range l.integration.ConfigSchema {
		fieldPath := "config_schema." + key
		if !fieldNameRE.MatchString(key) {
			l.add(fieldPath, LintError, "key must be snake_case")
		}
		switch field.Type {
		case "string", "integer", "boolean", "number", "array", "object":
		case "":
			l.add(fieldPath+".type", LintError, "is required")
		default:
			l.add(fieldPath+".type", LintError, "is not supported")
		}
	}
}

func (l *integrationLinter) tools() {
	seen := map[string]struct{}{}
	for idx, tool := range l.integration.Tools {
		fieldPath := fmt.Sprintf("tools[%d]", idx)
		if strings.TrimSpace(tool) == "" {
			l.add(fieldPath, LintError, "is required")
			continue
		}
		if !toolNameRE.MatchString(tool) {
			l.add(fieldPath, LintError, "must be snake_case or kebab-case")
		}
		if _, ok := seen[tool]; ok {
			l.add(fieldPath, LintError, "duplicates tool "+tool)
		}
		seen[tool] = struct{}{}
	}
}

func (l *integrationLinter) docsURL() {
	if l.integration.DocsURL == "" {
		return
	}
	parsed, err := url.Parse(l.integration.DocsURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		l.add("docs_url", LintError, "must be an absolute URL")
	}
}
