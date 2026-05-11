package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNullableString_NonEmpty_ReturnsString(t *testing.T) {
	v := nullableString("hello")
	assert.Equal(t, "hello", v)
}

func TestNullableString_Empty_ReturnsNil(t *testing.T) {
	v := nullableString("")
	assert.Nil(t, v)
}
