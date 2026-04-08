package plan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Parse decodes a raw JSON-encoded plan into a Plan struct.
// Returns an error if the JSON is invalid.
func Parse(rawPlan []byte) (Plan, error) {
	var plan Plan
	if err := json.Unmarshal(rawPlan, &plan); err != nil {
		return Plan{}, fmt.Errorf("failed to parse plan: %w", err)
	}
	return plan, nil
}

// Checksum computes the SHA-256 checksum of the raw plan bytes.
// Used for backward compatibility with orchestrators that compare applied-checksum.
func Checksum(rawPlan []byte) string {
	h := sha256.New()
	h.Write(rawPlan)
	return fmt.Sprintf("%x", h.Sum(nil))
}
