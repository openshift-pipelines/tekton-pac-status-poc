# Konflux-shaped example

These fixtures use the real public Konflux `TEST_OUTPUT` contract, with
synthetic metadata and no live tenant data.

Konflux's [`make_result_json`](https://github.com/konflux-ci/konflux-test/blob/4a9139ff5416841dc1e317eef36f4effa17c0cea/test/utils.sh#L8-L84)
produces these fields:

```json
{
  "result": "WARNING",
  "timestamp": "2026-10-05T09:00:00+00:00",
  "note": "Task deprecated-image-check completed: Check result for task result.",
  "namespace": "required_checks",
  "successes": 0,
  "failures": 0,
  "warnings": 1
}
```

The public
[`deprecated-image-check`](https://github.com/konflux-ci/build-definitions/blob/5d2cc53ce0484d2a7b35c7df3bd4f5d9c34423db/archived-tasks/deprecated-image-check/0.5/deprecated-image-check.yaml#L185-L215)
emits that JSON as a `TEST_OUTPUT` Task result while exiting successfully for
`SUCCESS`, `WARNING`, `FAILURE`, and `ERROR` outcomes.

## Current input

`taskrun-warning.json` and `taskrun-error.json` show what the completed Konflux
TaskRun looks like. Notice that both TaskRuns have `Succeeded=True`; the logical
outcome is inside the JSON string stored in `TEST_OUTPUT`.

```sh
jq -r '.status.results[] | select(.name == "TEST_OUTPUT").value | fromjson' \
  examples/konflux/taskrun-error.json
```

That produces:

```json
{
  "result": "ERROR",
  "timestamp": "2026-10-05T09:00:00+00:00",
  "note": "Task deprecated-image-check failed: Command conftest failed. For details, check Tekton task log.",
  "namespace": "required_checks",
  "successes": 0,
  "failures": 0,
  "warnings": 0
}
```

## PoC output

The final task in `tekton/pipelinerun.yaml` reduces `TEST_OUTPUT` to the bounded
`pac-status` PipelineResult. The PipelineRun fixtures show what PaC receives
after that aggregation.

For a warning:

```sh
go run . -provider github-checks examples/konflux/pipelinerun-warning.json
```

```json
{
  "executionConclusion": "success",
  "vcsConclusion": "neutral",
  "warning": true,
  "summary": "Task deprecated-image-check completed: Check result for task result."
}
```

GitLab has no neutral warning state in this PoC, so it remains successful while
carrying the warning details:

```sh
go run . -provider gitlab examples/konflux/pipelinerun-warning.json
```

```json
{
  "executionConclusion": "success",
  "vcsConclusion": "success",
  "warning": true,
  "summary": "Task deprecated-image-check completed: Check result for task result."
}
```

For `ERROR` or `FAILURE`, the final aggregation task exits non-zero. PaC then
uses the failed PipelineRun condition normally:

```sh
go run . -provider github-checks examples/konflux/pipelinerun-error.json
```

```json
{
  "executionConclusion": "failure",
  "vcsConclusion": "failure",
  "warning": false
}
```
