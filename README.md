# Tekton PaC status-result PoC

A small proof of concept for presenting a successful Tekton `PipelineRun` as a
warning in a Git provider without adding a new Tekton API.

The PoC uses existing primitives:

1. A task emits an existing-style `TEST_OUTPUT` result.
2. A final task reduces that result to one bounded `PipelineResult`.
3. `failure` or `error` makes the final task fail, so Kubernetes and the VCS
   agree that the `PipelineRun` failed.
4. `warning` leaves the `PipelineRun` successful for PaC to present.
5. The `PipelineRun` opts in by naming the reduced result in an annotation.

```yaml
metadata:
  annotations:
    pipelinesascode.tekton.dev/results-status: pac-status
```

The checked-in example replays the warning observed in a public Konflux run:

```json
{"result":"WARNING","note":"","successes":150,"failures":0,"warnings":5}
```

The final task reduces it to the bounded PaC-facing result:

```json
{"version":"1","outcome":"warning","summary":"Warning: 150 successes, 5 warnings"}
```

## Decision rules

The real `PipelineRun` condition always wins for failure and cancellation.
A result can add warning presentation to a successful run, but cannot turn a
successful run into a failure. Missing, malformed, oversized, or unsupported
results preserve the normal `PipelineRun` conclusion and produce a diagnostic.

| Provider API | Warning presentation |
|---|---|
| GitHub Check Runs | `neutral` |
| GitHub commit statuses | `success` plus warning text |
| GitLab commit statuses | `success` plus warning text |

GitHub commit statuses have no neutral state. PaC currently maps neutral to
success there. PaC currently maps neutral to canceled on GitLab, so this PoC
keeps the GitLab status successful and carries the warning separately rather
than claiming the run was canceled.

## Run locally

Requires Go 1.23 or newer.

```sh
make test
make demo

go run . -provider github-checks examples/warning.json
go run . -provider github-status examples/warning.json
go run . -provider gitlab examples/warning.json
```

Expected GitHub Check Runs decision:

```json
{
  "executionConclusion": "success",
  "vcsConclusion": "neutral",
  "warning": true,
  "summary": "A dependency is deprecated"
}
```

`examples/inconsistent-failure.json` demonstrates the safety rule: a result
cannot silently make VCS failure disagree with a successful `PipelineRun`.

## Public Konflux runs used as tests

[`examples/konflux`](examples/konflux) records the exact semantic output used by
the PoC tests:

- [`operator` check run 111713232004](https://github.com/openshift-pipelines/operator/runs/111713232004):
  `WARNING`, 150 successes, 5 warnings, GitHub conclusion `neutral`;
- [`syncer-service` check run 111703203017](https://github.com/openshift-pipelines/syncer-service/runs/111703203017):
  `FAILURE`, 295 successes, 99 warnings, 20 failures, GitHub conclusion
  `failure`.

`TestObservedKonfluxOutputs` verifies the reconstructed `TEST_OUTPUT` values and
uses the public Check Run conclusions as the expected provider output. The
fixture README documents exactly which fields are observed and which TaskRun
fields are reconstructed because direct cluster access requires authentication.

```sh
go test -run TestObservedKonfluxOutputs -v
make demo-konflux
```

## Run the Tekton example

The default manifest replays the observed 150-success/5-warning result and
finishes successfully:

```sh
run=$(kubectl create -f tekton/pipelinerun.yaml -o name)
kubectl wait --for=condition=Succeeded --timeout=5m "$run"
kubectl get "$run" -o json | go run . -provider github-checks -
```

Replace the `test-output` parameter with the `TEST_OUTPUT` value from
`examples/konflux/taskrun-failure.json` to prove that the final task makes the
`PipelineRun` fail.

## Upstream integration point

PaC currently derives the final conclusion only from the `PipelineRun`
condition in
[`pkg/formatting/pipelinerun.go`](https://github.com/openshift-pipelines/pipelines-as-code/blob/f2a0c748516af33b82218a1d0b72839380386461/pkg/formatting/pipelinerun.go#L10-L21)
and passes it to providers from
[`pkg/reconciler/status.go`](https://github.com/openshift-pipelines/pipelines-as-code/blob/f2a0c748516af33b82218a1d0b72839380386461/pkg/reconciler/status.go#L109-L120).
The proposed integration is one result-policy function at that boundary.

Provider constraints are visible in PaC's
[GitHub commit-status mapping](https://github.com/openshift-pipelines/pipelines-as-code/blob/f2a0c748516af33b82218a1d0b72839380386461/pkg/provider/github/status.go#L463-L478)
and
[GitLab status mapping](https://github.com/openshift-pipelines/pipelines-as-code/blob/f2a0c748516af33b82218a1d0b72839380386461/pkg/provider/gitlab/gitlab.go#L508-L525).

## Scope

This repository proves the result contract and provider decision matrix. It
does not call Git provider APIs or patch PaC yet. If the contract is accepted,
the next step is to move the tested decision function into PaC and add provider
rendering tests.
