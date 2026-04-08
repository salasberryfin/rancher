package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPlanJSONRoundtrip verifies that a fully-populated Plan
// serializes and deserializes without data loss.
func TestPlanJSONRoundtrip(t *testing.T) {
	original := Plan{
		Files: []File{
			{
				Content:     "aGVsbG8=", // base64 "hello"
				Directory:   false,
				UID:         1000,
				GID:         1000,
				Path:        "/etc/myapp/config.yaml",
				Permissions: "0644",
				Action:      "",
			},
			{
				Path:      "/etc/myapp/",
				Directory: true,
			},
		},
		OneTimeInstructions: []OneTimeInstruction{
			{
				CommonInstruction: CommonInstruction{
					Name:    "install",
					Command: "/bin/sh",
					Args:    []string{"-c", "echo hello"},
					Env:     []string{"MY_VAR=value"},
					Image:   "alpine:latest",
				},
				SaveOutput: true,
			},
		},
		PeriodicInstructions: []PeriodicInstruction{
			{
				CommonInstruction: CommonInstruction{
					Name:    "healthcheck",
					Command: "/bin/sh",
					Args:    []string{"-c", "echo ok"},
				},
				PeriodSeconds:    60,
				SaveStderrOutput: true,
			},
		},
		Probes: map[string]Probe{
			"api": {
				Name:                "api",
				InitialDelaySeconds: 5,
				TimeoutSeconds:      3,
				SuccessThreshold:    1,
				FailureThreshold:    3,
				HTTPGetAction: HTTPGetAction{
					URL:        "https://localhost:6443/healthz",
					Insecure:   false,
					CACert:     "/etc/ssl/ca.pem",
					ClientCert: "/etc/ssl/client.crt",
					ClientKey:  "/etc/ssl/client.key",
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal Plan: %v", err)
	}

	var restored Plan
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal Plan: %v", err)
	}

	if len(restored.Files) != len(original.Files) {
		t.Errorf("Files: want %d, got %d", len(original.Files), len(restored.Files))
	}
	if restored.Files[0].Path != original.Files[0].Path {
		t.Errorf("Files[0].Path: want %q, got %q", original.Files[0].Path, restored.Files[0].Path)
	}
	if restored.Files[0].Content != original.Files[0].Content {
		t.Errorf("Files[0].Content: want %q, got %q", original.Files[0].Content, restored.Files[0].Content)
	}
	if len(restored.OneTimeInstructions) != 1 {
		t.Fatalf("OneTimeInstructions: want 1, got %d", len(restored.OneTimeInstructions))
	}
	if restored.OneTimeInstructions[0].Name != "install" {
		t.Errorf("OneTimeInstructions[0].Name: want %q, got %q", "install", restored.OneTimeInstructions[0].Name)
	}
	if !restored.OneTimeInstructions[0].SaveOutput {
		t.Error("OneTimeInstructions[0].SaveOutput: want true, got false")
	}
	probe, ok := restored.Probes["api"]
	if !ok {
		t.Fatal("Probes[api]: missing after roundtrip")
	}
	if probe.HTTPGetAction.URL != "https://localhost:6443/healthz" {
		t.Errorf("Probes[api].HTTPGetAction.URL: want %q, got %q",
			"https://localhost:6443/healthz", probe.HTTPGetAction.URL)
	}
	if len(restored.PeriodicInstructions) != 1 {
		t.Fatalf("PeriodicInstructions: want 1, got %d", len(restored.PeriodicInstructions))
	}
	if restored.PeriodicInstructions[0].PeriodSeconds != 60 {
		t.Errorf("PeriodicInstructions[0].PeriodSeconds: want 60, got %d", restored.PeriodicInstructions[0].PeriodSeconds)
	}
}

// TestJSONTagsMatchContract verifies that JSON field names match the expected
// wire format used by the system-agent. These are the canonical key names —
// changing them is a breaking change.
func TestJSONTagsMatchContract(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		validate func(t *testing.T, p Plan)
	}{
		{
			name:  "files key",
			input: `{"files":[{"path":"/tmp/test","content":"aGk="}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.Files) != 1 || p.Files[0].Path != "/tmp/test" {
					t.Errorf("expected Files[0].Path=/tmp/test, got %+v", p.Files)
				}
			},
		},
		{
			name:  "instructions key (not oneTimeInstructions)",
			input: `{"instructions":[{"name":"test","command":"/bin/sh"}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.OneTimeInstructions) != 1 || p.OneTimeInstructions[0].Name != "test" {
					t.Errorf("expected OneTimeInstructions[0].Name=test, got %+v", p.OneTimeInstructions)
				}
			},
		},
		{
			name:  "periodicInstructions key",
			input: `{"periodicInstructions":[{"name":"cron","periodSeconds":300}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.PeriodicInstructions) != 1 || p.PeriodicInstructions[0].PeriodSeconds != 300 {
					t.Errorf("expected PeriodicInstructions[0].PeriodSeconds=300, got %+v", p.PeriodicInstructions)
				}
			},
		},
		{
			name:  "probes key with httpGet",
			input: `{"probes":{"web":{"httpGet":{"url":"http://localhost/health","insecure":true}}}}`,
			validate: func(t *testing.T, p Plan) {
				probe, ok := p.Probes["web"]
				if !ok {
					t.Fatal("probes.web missing")
				}
				if probe.HTTPGetAction.URL != "http://localhost/health" {
					t.Errorf("expected httpGet.url=http://localhost/health, got %q", probe.HTTPGetAction.URL)
				}
				if !probe.HTTPGetAction.Insecure {
					t.Error("expected httpGet.insecure=true")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p Plan
			if err := json.Unmarshal([]byte(tc.input), &p); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			tc.validate(t, p)
		})
	}
}

// TestEmptyPlanMarshalsCleanly ensures an empty Plan produces minimal JSON
// (all fields have omitempty) and round-trips without error.
func TestEmptyPlanMarshalsCleanly(t *testing.T) {
	p := Plan{}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("expected empty plan to marshal to {}, got: %s", data)
	}
}

// TestFileActionDelete verifies the "delete" action is preserved through JSON.
func TestFileActionDelete(t *testing.T) {
	f := File{Path: "/tmp/remove-me", Action: "delete"}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var restored File
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if restored.Action != "delete" {
		t.Errorf("Action: want %q, got %q", "delete", restored.Action)
	}
}

// TestProbeStatusRoundtrip verifies ProbeStatus serializes correctly.
func TestProbeStatusRoundtrip(t *testing.T) {
	ps := ProbeStatus{Healthy: true, SuccessCount: 3, FailureCount: 0}
	data, err := json.Marshal(ps)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var restored ProbeStatus
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !restored.Healthy || restored.SuccessCount != 3 {
		t.Errorf("unexpected ProbeStatus after roundtrip: %+v", restored)
	}
}

// TestParseValidPlan verifies that Parse correctly deserializes a valid JSON plan.
func TestParseValidPlan(t *testing.T) {
	input := `{"files":[{"path":"/tmp/test","content":"aGk="}],"instructions":[{"name":"setup","command":"/bin/sh"}]}`
	p, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(p.Files) != 1 || p.Files[0].Path != "/tmp/test" {
		t.Errorf("Files not parsed correctly: %+v", p.Files)
	}
	if len(p.OneTimeInstructions) != 1 || p.OneTimeInstructions[0].Name != "setup" {
		t.Errorf("OneTimeInstructions not parsed correctly: %+v", p.OneTimeInstructions)
	}
}

// TestParseInvalidJSON verifies that Parse returns an error for invalid JSON.
func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse plan") {
		t.Errorf("expected error to contain 'failed to parse plan', got: %v", err)
	}
}

// TestChecksum verifies that Checksum returns a deterministic hex string for known input.
func TestChecksum(t *testing.T) {
	input := []byte(`{"files":[]}`)
	sum1 := Checksum(input)
	sum2 := Checksum(input)

	if sum1 != sum2 {
		t.Errorf("Checksum is not deterministic: %q != %q", sum1, sum2)
	}
	// SHA-256 hex is always 64 characters
	if len(sum1) != 64 {
		t.Errorf("expected 64-char hex string, got %d chars: %q", len(sum1), sum1)
	}
	// Different inputs must produce different checksums
	other := Checksum([]byte(`{"files":[],"instructions":[]}`))
	if sum1 == other {
		t.Error("expected different checksums for different inputs")
	}
}

// TestPlanStateConstants spot-checks that the PlanState constants have the correct values.
func TestPlanStateConstants(t *testing.T) {
	cases := []struct {
		state    PlanState
		expected string
	}{
		{PlanStatePending, "pending"},
		{PlanStateInProgress, "in-progress"},
		{PlanStateSucceeded, "succeeded"},
		{PlanStateFailed, "failed"},
		{PlanStateCancelled, "cancelled"},
	}
	for _, tc := range cases {
		if string(tc.state) != tc.expected {
			t.Errorf("PlanState constant: want %q, got %q", tc.expected, tc.state)
		}
	}
}
