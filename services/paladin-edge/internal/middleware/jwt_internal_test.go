package middleware

import "testing"

// TestAuthError_Error verifies the authError type's Error() method returns its
// underlying message. authError is unexported, so this test lives in the
// internal test package.
func TestAuthError_Error(t *testing.T) {
	t.Parallel()
	e := &authError{msg: "boom"}
	if got := e.Error(); got != "boom" {
		t.Fatalf("authError.Error() = %q, want %q", got, "boom")
	}
}

// TestErrMissing_ExposesMessage ensures the package-level errMissing sentinel
// carries the expected user-facing message via authError.Error().
func TestErrMissing_ExposesMessage(t *testing.T) {
	t.Parallel()
	if errMissing.Error() != "missing authorization header" {
		t.Fatalf("unexpected errMissing message: %q", errMissing.Error())
	}
}
