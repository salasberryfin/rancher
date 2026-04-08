package plan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPlanRoundtrip(t *testing.T) {
	original := Plan{
		Files: []File{
			{
				Content:     "aGVsbG8=", // base64 "hello"
				UID:         1000,
				GID:         1000,
				Path:        "/etc/myapp/config.yaml",
				Permissions: "0644",
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
					CACert:     "/etc/ssl/ca.pem",
					ClientCert: "/etc/ssl/client.crt",
					ClientKey:  "/etc/ssl/client.key",
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored Plan
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, restored) {
		t.Errorf("plan changed after roundtrip\nwant: %+v\n got: %+v", original, restored)
	}
}

// TestJSONKeys checks the JSON field names that form the wire contract with the
// system-agent. These are load-bearing — a rename is a breaking change.
func TestJSONKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(t *testing.T, p Plan)
	}{
		{
			name:  "files",
			input: `{"files":[{"path":"/tmp/test","content":"aGk="}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.Files) != 1 || p.Files[0].Path != "/tmp/test" {
					t.Errorf("got %+v", p.Files)
				}
			},
		},
		{
			// OneTimeInstructions is serialised as "instructions", not "oneTimeInstructions"
			name:  "instructions",
			input: `{"instructions":[{"name":"test","command":"/bin/sh"}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.OneTimeInstructions) != 1 || p.OneTimeInstructions[0].Name != "test" {
					t.Errorf("got %+v", p.OneTimeInstructions)
				}
			},
		},
		{
			name:  "periodicInstructions",
			input: `{"periodicInstructions":[{"name":"cron","periodSeconds":300}]}`,
			validate: func(t *testing.T, p Plan) {
				if len(p.PeriodicInstructions) != 1 || p.PeriodicInstructions[0].PeriodSeconds != 300 {
					t.Errorf("got %+v", p.PeriodicInstructions)
				}
			},
		},
		{
			name:  "probes / httpGet",
			input: `{"probes":{"web":{"httpGet":{"url":"http://localhost/health","insecure":true}}}}`,
			validate: func(t *testing.T, p Plan) {
				probe, ok := p.Probes["web"]
				if !ok {
					t.Fatal("probes.web missing")
				}
				if probe.HTTPGetAction.URL != "http://localhost/health" || !probe.HTTPGetAction.Insecure {
					t.Errorf("got %+v", probe.HTTPGetAction)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p Plan
			if err := json.Unmarshal([]byte(tt.input), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			tt.validate(t, p)
		})
	}
}

func TestEmptyPlanJSON(t *testing.T) {
	// All fields carry omitempty, so an empty Plan must produce "{}".
	data, err := json.Marshal(Plan{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("want {}, got %s", data)
	}
}

func TestFileDeleteAction(t *testing.T) {
	f := File{Path: "/tmp/remove-me", Action: "delete"}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out File
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Action != "delete" {
		t.Errorf("want action=delete, got %q", out.Action)
	}
}

func TestProbeStatusRoundtrip(t *testing.T) {
	ps := ProbeStatus{Healthy: true, SuccessCount: 3}
	data, err := json.Marshal(ps)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out ProbeStatus
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(ps, out) {
		t.Errorf("want %+v, got %+v", ps, out)
	}
}

func TestParse(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		raw := `{"files":[{"path":"/tmp/test","content":"aGk="}],"instructions":[{"name":"setup","command":"/bin/sh"}]}`
		p, err := Parse([]byte(raw))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.Files) != 1 || p.Files[0].Path != "/tmp/test" {
			t.Errorf("unexpected files: %+v", p.Files)
		}
		if len(p.OneTimeInstructions) != 1 || p.OneTimeInstructions[0].Name != "setup" {
			t.Errorf("unexpected instructions: %+v", p.OneTimeInstructions)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := Parse([]byte(`{not valid json`))
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "failed to parse plan") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestChecksum(t *testing.T) {
	input := []byte(`{"files":[]}`)

	// same input → same output, always
	if Checksum(input) != Checksum(input) {
		t.Error("checksum is not deterministic")
	}

	// SHA-256 hex digest is 64 characters
	if got := len(Checksum(input)); got != 64 {
		t.Errorf("expected 64-char hex digest, got %d", got)
	}

	// different input → different output
	if Checksum(input) == Checksum([]byte(`{"files":[],"instructions":[]}`)) {
		t.Error("different inputs produced the same checksum")
	}
}

func TestPlanStateValues(t *testing.T) {
	if PlanStatePending != "pending" {
		t.Errorf("PlanStatePending = %q", PlanStatePending)
	}
	if PlanStateInProgress != "in-progress" {
		t.Errorf("PlanStateInProgress = %q", PlanStateInProgress)
	}
	if PlanStateSucceeded != "succeeded" {
		t.Errorf("PlanStateSucceeded = %q", PlanStateSucceeded)
	}
	if PlanStateFailed != "failed" {
		t.Errorf("PlanStateFailed = %q", PlanStateFailed)
	}
	if PlanStateCancelled != "cancelled" {
		t.Errorf("PlanStateCancelled = %q", PlanStateCancelled)
	}
}
