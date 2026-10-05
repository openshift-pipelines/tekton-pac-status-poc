package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	warning := `{"version":"1","outcome":"warning","summary":"dependency is deprecated"}`
	tests := []struct {
		name       string
		provider   string
		input      string
		want       decision
		diagnostic string
	}{
		{
			name:     "successful PipelineRun without opt in",
			provider: "github-checks",
			input:    pipelineRunJSON("True", "Succeeded", "", ""),
			want:     decision{ExecutionConclusion: "success", VCSConclusion: "success"},
		},
		{
			name:     "warning is neutral in GitHub Checks",
			provider: "github-checks",
			input:    pipelineRunJSON("True", "Succeeded", "pac-status", warning),
			want: decision{
				ExecutionConclusion: "success",
				VCSConclusion:       "neutral",
				Warning:             true,
				Summary:             "dependency is deprecated",
			},
		},
		{
			name:     "warning remains success in GitHub commit status",
			provider: "github-status",
			input:    pipelineRunJSON("True", "Succeeded", "pac-status", warning),
			want: decision{
				ExecutionConclusion: "success",
				VCSConclusion:       "success",
				Warning:             true,
				Summary:             "dependency is deprecated",
			},
		},
		{
			name:     "warning remains success in GitLab",
			provider: "gitlab",
			input:    pipelineRunJSON("True", "Succeeded", "pac-status", warning),
			want: decision{
				ExecutionConclusion: "success",
				VCSConclusion:       "success",
				Warning:             true,
				Summary:             "dependency is deprecated",
			},
		},
		{
			name:     "failed PipelineRun always fails",
			provider: "github-checks",
			input:    pipelineRunJSON("False", "Failed", "pac-status", warning),
			want:     decision{ExecutionConclusion: "failure", VCSConclusion: "failure"},
		},
		{
			name:       "missing configured result falls back",
			provider:   "github-checks",
			input:      pipelineRunJSON("True", "Succeeded", "other-result", warning),
			want:       decision{ExecutionConclusion: "success", VCSConclusion: "success"},
			diagnostic: "was not present",
		},
		{
			name:       "malformed configured result falls back",
			provider:   "github-checks",
			input:      pipelineRunJSON("True", "Succeeded", "pac-status", `{not-json}`),
			want:       decision{ExecutionConclusion: "success", VCSConclusion: "success"},
			diagnostic: "was invalid",
		},
		{
			name:       "result cannot turn successful execution into failure",
			provider:   "github-checks",
			input:      pipelineRunJSON("True", "Succeeded", "pac-status", `{"version":"1","outcome":"failure","summary":"tests failed"}`),
			want:       decision{ExecutionConclusion: "success", VCSConclusion: "success", Summary: "tests failed"},
			diagnostic: "final task must fail",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var pr pipelineRun
			if err := json.Unmarshal([]byte(test.input), &pr); err != nil {
				t.Fatal(err)
			}
			got := decide(pr, test.provider)
			if test.diagnostic == "" {
				if got != test.want {
					t.Fatalf("decide() = %#v, want %#v", got, test.want)
				}
				return
			}
			if !strings.Contains(got.Diagnostic, test.diagnostic) {
				t.Fatalf("diagnostic %q does not contain %q", got.Diagnostic, test.diagnostic)
			}
			got.Diagnostic = ""
			if got != test.want {
				t.Fatalf("decide() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestKonfluxExamples(t *testing.T) {
	tests := []struct {
		name string
		file string
		want decision
	}{
		{
			name: "warning",
			file: "examples/konflux/pipelinerun-warning.json",
			want: decision{
				ExecutionConclusion: "success",
				VCSConclusion:       "neutral",
				Warning:             true,
				Summary:             "Task deprecated-image-check completed: Check result for task result.",
			},
		},
		{
			name: "error",
			file: "examples/konflux/pipelinerun-error.json",
			want: decision{ExecutionConclusion: "failure", VCSConclusion: "failure"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(test.file)
			if err != nil {
				t.Fatal(err)
			}
			var pr pipelineRun
			if err := json.Unmarshal(data, &pr); err != nil {
				t.Fatal(err)
			}
			if got := decide(pr, "github-checks"); got != test.want {
				t.Fatalf("decide() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func pipelineRunJSON(condition, reason, resultName, resultValue string) string {
	annotations := "{}"
	results := "[]"
	if resultName != "" {
		annotations = `{"pipelinesascode.tekton.dev/results-status":"pac-status"}`
		encoded, _ := json.Marshal(resultValue)
		results = `[{"name":"` + resultName + `","value":` + string(encoded) + `}]`
	}
	return `{
		"metadata":{"annotations":` + annotations + `},
		"status":{
			"conditions":[{"type":"Succeeded","status":"` + condition + `","reason":"` + reason + `"}],
			"results":` + results + `
		}
	}`
}
