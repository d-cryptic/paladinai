package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCORSOrigins_DefaultsWhenEmpty(t *testing.T) {
	got := parseCORSOrigins("")
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3001"}, got)
}

func TestParseCORSOrigins_Single(t *testing.T) {
	got := parseCORSOrigins("https://app.example.com")
	assert.Equal(t, []string{"https://app.example.com"}, got)
}

func TestParseCORSOrigins_Multiple(t *testing.T) {
	got := parseCORSOrigins("https://app.example.com,https://admin.example.com")
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, got)
}

func TestParseCORSOrigins_TrimsSpaces(t *testing.T) {
	got := parseCORSOrigins("  https://a.com , https://b.com  ")
	assert.Equal(t, []string{"https://a.com", "https://b.com"}, got)
}

func TestParseCORSOrigins_AllBlankFallsBackToDefault(t *testing.T) {
	got := parseCORSOrigins("   ,   ")
	assert.Equal(t, []string{"http://localhost:3000", "http://localhost:3001"}, got)
}
