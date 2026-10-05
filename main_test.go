package main

import (
	"encoding/json"
	"fmt"
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

type observedTaskResult struct {
	Name      string `json:"name"`
	Result    string `json:"result"`
	Note      string `json:"note"`
	Successes int    `json:"successes"`
	Failures  int    `json:"failures"`
	Warnings  int    `json:"warnings"`
}

type observedKonfluxRun struct {
	Source struct {
		URL string `json:"url"`
	} `json:"source"`
	GitHubCheck struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
	} `json:"githubCheck"`
	TaskResult observedTaskResult `json:"taskResult"`
}

func TestObservedKonfluxOutputs(t *testing.T) {
	tests := []struct {
		name        string
		observed    string
		taskRun     string
		pipelineRun string
	}{
		{
			name:        "warning",
			observed:    "examples/konflux/observed-warning.json",
			taskRun:     "examples/konflux/taskrun-warning.json",
			pipelineRun: "examples/konflux/pipelinerun-warning.json",
		},
		{
			name:        "failure",
			observed:    "examples/konflux/observed-failure.json",
			taskRun:     "examples/konflux/taskrun-failure.json",
			pipelineRun: "examples/konflux/pipelinerun-failure.json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var observed observedKonfluxRun
			readJSONFixture(t, test.observed, &observed)
			if observed.Source.URL == "" || observed.GitHubCheck.Status != "completed" {
				t.Fatalf("invalid observed source: %#v", observed)
			}

			if got := readTaskResultFixture(t, test.taskRun); got != observed.TaskResult {
				t.Fatalf("TEST_OUTPUT = %#v, observed %#v", got, observed.TaskResult)
			}

			var pr pipelineRun
			readJSONFixture(t, test.pipelineRun, &pr)
			got := decide(pr, "github-checks")
			if got.VCSConclusion != observed.GitHubCheck.Conclusion {
				t.Fatalf("VCS conclusion = %q, observed %q", got.VCSConclusion, observed.GitHubCheck.Conclusion)
			}
			if got.Warning != (observed.GitHubCheck.Title == "Warning") {
				t.Fatalf("warning = %t, observed title %q", got.Warning, observed.GitHubCheck.Title)
			}

			for _, count := range []struct {
				value int
				label string
			}{
				{observed.TaskResult.Successes, "successes"},
				{observed.TaskResult.Warnings, "warnings"},
				{observed.TaskResult.Failures, "failures"},
			} {
				if got.Warning && count.value > 0 && !strings.Contains(got.Summary, fmt.Sprintf("%d %s", count.value, count.label)) {
					t.Fatalf("summary %q does not contain observed count %d %s", got.Summary, count.value, count.label)
				}
			}
		})
	}
}

func readJSONFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatal(err)
	}
}

func readTaskResultFixture(t *testing.T, path string) observedTaskResult {
	t.Helper()
	var taskRun struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Status struct {
			Results []struct {
				Name  string          `json:"name"`
				Value json.RawMessage `json:"value"`
			} `json:"results"`
		} `json:"status"`
	}
	readJSONFixture(t, path, &taskRun)
	for _, result := range taskRun.Status.Results {
		if result.Name != "TEST_OUTPUT" {
			continue
		}
		var encoded string
		if err := json.Unmarshal(result.Value, &encoded); err != nil {
			t.Fatal(err)
		}
		var output observedTaskResult
		if err := json.Unmarshal([]byte(encoded), &output); err != nil {
			t.Fatal(err)
		}
		output.Name = taskRun.Metadata.Labels["tekton.dev/pipelineTask"]
		return output
	}
	t.Fatal("TEST_OUTPUT not found")
	return observedTaskResult{}
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
