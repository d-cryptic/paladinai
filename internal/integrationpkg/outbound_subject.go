package integrationpkg

import (
	"fmt"
	"strings"
)

func outboundDLQSubject(integration, tenantID string) (string, error) {
	if err := validateOutboundSubjectToken("integration", integration); err != nil {
		return "", err
	}
	if err := validateOutboundSubjectToken("tenantID", tenantID); err != nil {
		return "", err
	}
	return fmt.Sprintf("dlq.outbound.%s.%s", integration, tenantID), nil
}

func validateOutboundSubjectToken(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.ContainsAny(value, ".*>") {
		return fmt.Errorf("%s must not contain '.', '*', or '>'", name)
	}
	return nil
}
