# Konflux runs used by the PoC tests

The tests in this repository are grounded in two public Konflux Check Runs from
the `openshift-pipelines` organization.

| Case | Public run | Observed task output | GitHub Check output |
|---|---|---|---|
| Warning | [`operator` PR #32549](https://github.com/openshift-pipelines/operator/pull/32549), [check run 111713232004](https://github.com/openshift-pipelines/operator/runs/111713232004) | `verify`: `WARNING`; 150 successes, 0 failures, 5 warnings | title `Warning`; conclusion `neutral` |
| Failure | [`syncer-service` PR #115](https://github.com/openshift-pipelines/syncer-service/pull/115), [check run 111703203017](https://github.com/openshift-pipelines/syncer-service/runs/111703203017) | `verify`: `FAILURE`; 295 successes, 20 failures, 99 warnings | title `Failed`; conclusion `failure` |

`observed-warning.json` and `observed-failure.json` preserve the public source,
commit, Check Run conclusion, title, summary, and rendered task-result counts.
Environment-specific cluster links were omitted.

The observations can be reproduced with the public GitHub API:

```sh
gh api repos/openshift-pipelines/operator/check-runs/111713232004 \
  --jq '{name,status,conclusion,output}'

gh api repos/openshift-pipelines/syncer-service/check-runs/111703203017 \
  --jq '{name,status,conclusion,output}'
```

## What is reconstructed

The Konflux Kubernetes API requires authentication, so these public Check Runs
do not expose the raw TaskRun objects. `taskrun-warning.json` and
`taskrun-failure.json` are therefore sanitized TaskRun-shaped reconstructions,
not cluster exports. They use:

- the exact result and counters rendered by the public Check Run;
- the public Konflux [`TEST_OUTPUT` schema](https://github.com/konflux-ci/integration-service/blob/4bea52e03bd58739f43dc0a29eb143c97fdddd4f/helpers/integration.go#L45-L119);
- the Check Run completion time as the fixture timestamp.

The blank `note` and test-suite fields match the rendered Check output. The PoC
therefore builds a bounded summary from the counters instead of inventing a
note.

`pipelinerun-warning.json` and `pipelinerun-failure.json` show the corresponding
PipelineRun state after the final aggregation task. For `FAILURE`, that task
exits non-zero so the PipelineRun and VCS conclusions remain aligned.

## How the fixtures are tested

`TestObservedKonfluxOutputs` in `main_test.go`:

1. loads each `observed-*.json` public observation;
2. verifies the reconstructed TaskRun has the same result and counters;
3. runs the PaC decision over the corresponding PipelineRun;
4. checks its GitHub conclusion against the observed Check Run; and
5. checks that warning details retain the observed counts.

Run it with:

```sh
go test -run TestObservedKonfluxOutputs -v
```

The warning decision is:

```json
{
  "executionConclusion": "success",
  "vcsConclusion": "neutral",
  "warning": true,
  "summary": "Warning: 150 successes, 5 warnings"
}
```

The failure decision is:

```json
{
  "executionConclusion": "failure",
  "vcsConclusion": "failure",
  "warning": false
}
```

## Existing Konflux behavior

This is the same decision policy already used by Konflux Integration Service:

- it reads `TEST_OUTPUT` from child TaskRuns and treats `WARNING` as passing
  with warnings ([aggregation](https://github.com/konflux-ci/integration-service/blob/4bea52e03bd58739f43dc0a29eb143c97fdddd4f/helpers/integration.go#L251-L284));
- it renders the result and counters into the task table
  ([formatting](https://github.com/konflux-ci/integration-service/blob/4bea52e03bd58739f43dc0a29eb143c97fdddd4f/status/format.go#L160-L252));
- it maps warnings to GitHub Check Runs `neutral` and GitHub commit statuses
  `success` ([GitHub mapping](https://github.com/konflux-ci/integration-service/blob/4bea52e03bd58739f43dc0a29eb143c97fdddd4f/status/reporter_github.go#L568-L618)); and
- it maps warnings to GitLab `success`
  ([GitLab mapping](https://github.com/konflux-ci/integration-service/blob/4bea52e03bd58739f43dc0a29eb143c97fdddd4f/status/reporter_gitlab.go#L650-L692)).
