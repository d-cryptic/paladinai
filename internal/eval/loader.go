package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadFixtures reads all *.jsonl files in dir (non-recursive) and returns the
// concatenated, validated TestCases. Files are loaded in sorted order so test
// runs are deterministic.
func LoadFixtures(dir string) ([]TestCase, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixtures dir %q: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)

	var all []TestCase
	seen := make(map[string]string) // id -> file
	for _, f := range files {
		cases, err := loadJSONLFile(f)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", f, err)
		}
		for _, tc := range cases {
			if prev, ok := seen[tc.ID]; ok {
				return nil, fmt.Errorf("duplicate test id %q in %s (first seen in %s)", tc.ID, f, prev)
			}
			seen[tc.ID] = f
			all = append(all, tc)
		}
	}
	return all, nil
}

func loadJSONLFile(path string) ([]TestCase, error) {
	f, err := os.Open(path) // #nosec G304 -- path is supplied by caller (CLI)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	var cases []TestCase
	scanner := bufio.NewScanner(f)
	// Some test cases may have long descriptions; bump the buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "//") {
			continue
		}
		var tc TestCase
		if err := json.Unmarshal([]byte(raw), &tc); err != nil {
			return nil, fmt.Errorf("line %d: parse: %w", line, err)
		}
		if err := validateCase(tc); err != nil {
			return nil, fmt.Errorf("line %d (%s): %w", line, tc.ID, err)
		}
		cases = append(cases, tc)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}
	return cases, nil
}

func validateCase(tc TestCase) error {
	if strings.TrimSpace(tc.ID) == "" {
		return fmt.Errorf("missing id")
	}
	if !tc.Category.Valid() {
		return fmt.Errorf("invalid category %q", tc.Category)
	}
	// Summary and tool_use cases may use context instead of alert.title.
	if tc.Category != CategorySummary && tc.Category != CategoryToolUse {
		if strings.TrimSpace(tc.Alert.Title) == "" {
			return fmt.Errorf("missing alert.title")
		}
	}
	if !tc.HasExpectations() {
		return fmt.Errorf("test case has no expected outputs")
	}
	return nil
}
