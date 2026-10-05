package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const statusResultAnnotation = "pipelinesascode.tekton.dev/results-status"

type pipelineRun struct {
	Metadata struct {
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Status struct {
		Conditions []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
		Results []struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		} `json:"results"`
	} `json:"status"`
}

type report struct {
	Version string `json:"version"`
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
}

type decision struct {
	ExecutionConclusion string `json:"executionConclusion"`
	VCSConclusion       string `json:"vcsConclusion"`
	Warning             bool   `json:"warning"`
	Summary             string `json:"summary,omitempty"`
	Diagnostic          string `json:"diagnostic,omitempty"`
}

func main() {
	provider := flag.String("provider", "github-checks", "github-checks, github-status, or gitlab")
	flag.Parse()
	if flag.NArg() > 1 {
		fatal(errors.New("usage: pac-status-poc [-provider provider] [pipelinerun.json]"))
	}
	if !validProvider(*provider) {
		fatal(fmt.Errorf("unsupported provider %q", *provider))
	}

	input := io.Reader(os.Stdin)
	if flag.NArg() == 1 && flag.Arg(0) != "-" {
		file, err := os.Open(flag.Arg(0))
		if err != nil {
			fatal(err)
		}
		defer file.Close()
		input = file
	}

	var pr pipelineRun
	decoder := json.NewDecoder(input)
	if err := decoder.Decode(&pr); err != nil {
		fatal(fmt.Errorf("decode PipelineRun: %w", err))
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		fatal(errors.New("input must contain exactly one JSON object"))
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(decide(pr, *provider)); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func validProvider(provider string) bool {
	return provider == "github-checks" || provider == "github-status" || provider == "gitlab"
}

func decide(pr pipelineRun, provider string) decision {
	base := pipelineConclusion(pr)
	result := decision{ExecutionConclusion: base, VCSConclusion: base}
	if base != "success" {
		return result
	}

	name := strings.TrimSpace(pr.Metadata.Annotations[statusResultAnnotation])
	if name == "" {
		return result
	}

	raw, ok := findResult(pr, name)
	if !ok {
		result.Diagnostic = fmt.Sprintf("configured status result %q was not present; using PipelineRun conclusion", name)
		return result
	}

	report, err := parseReport(raw)
	if err != nil {
		result.Diagnostic = fmt.Sprintf("configured status result %q was invalid: %v; using PipelineRun conclusion", name, err)
		return result
	}
	result.Summary = report.Summary

	switch report.Outcome {
	case "success":
		return result
	case "warning":
		result.Warning = true
		if provider == "github-checks" {
			result.VCSConclusion = "neutral"
		}
		return result
	case "failure", "error":
		result.Diagnostic = fmt.Sprintf("status result reported %q but the PipelineRun succeeded; a final task must fail the PipelineRun", report.Outcome)
		return result
	default:
		panic("validated outcome was not handled")
	}
}

func pipelineConclusion(pr pipelineRun) string {
	for _, condition := range pr.Status.Conditions {
		if condition.Type != "Succeeded" {
			continue
		}
		if strings.Contains(strings.ToLower(condition.Reason), "cancel") {
			return "cancelled"
		}
		switch condition.Status {
		case "True":
			return "success"
		case "False":
			return "failure"
		default:
			return "neutral"
		}
	}
	return "neutral"
}

func findResult(pr pipelineRun, name string) (json.RawMessage, bool) {
	for _, result := range pr.Status.Results {
		if result.Name == name {
			return result.Value, true
		}
	}
	return nil, false
}

func parseReport(raw json.RawMessage) (report, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return report{}, errors.New("empty value")
	}

	if raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return report{}, err
		}
		raw = []byte(value)
	}

	var value report
	if err := json.Unmarshal(raw, &value); err != nil {
		return report{}, err
	}
	value.Version = strings.TrimSpace(value.Version)
	value.Outcome = strings.ToLower(strings.TrimSpace(value.Outcome))
	value.Summary = strings.TrimSpace(value.Summary)

	if value.Version != "1" {
		return report{}, fmt.Errorf("unsupported version %q", value.Version)
	}
	switch value.Outcome {
	case "success", "warning", "failure", "error":
	default:
		return report{}, fmt.Errorf("unsupported outcome %q", value.Outcome)
	}
	if value.Summary == "" {
		return report{}, errors.New("summary is required")
	}
	if len(value.Summary) > 1024 {
		return report{}, errors.New("summary exceeds 1024 bytes")
	}
	return value, nil
}
