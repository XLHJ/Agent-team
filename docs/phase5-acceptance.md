# Phase 5 acceptance: sequential collaboration chain

- Status: delivered and verified 2026-11-13
- Scope: data model + sequential executor + persistence + panel API.
  The workflow editor canvas/run views remain gated on the graphic spec
  (chapter 9) and are intentionally NOT coded.

## Delivered

- hub/internal/workflows (schema/store/runs/stages/engine/chain/binding):
  - schema.go: 4 tables (workflows, workflow_stages, workflow_runs,
    workflow_run_stages) + indexes (doc 3.2); HandoffFormat (doc 4.1)
  - store.go: SaveWorkflow/ListWorkflows/GetWorkflow/DeleteWorkflow
  - runs.go: CreateRun/UpdateRunStatus/UpdateRunStage/GetRun/ListRuns/
    RunningRuns/CleanupRuns (90 day window)
  - stages.go: listRunStages/RunStages/SetRunStageTask
  - engine.go: Start/dispatchStage/failRun/StageCompleted/StageFailed/
    Cancel/Recover/OnTaskCompleted/OnTaskFailed
  - chain.go: facade; binding.go: SetBinding/SetDispatcher
- Handoff format (doc 4.1): input_context = prev report + next task_prompt;
  report_prompt rides as its own dispatch field
- Report validation (doc 4.2): missing report -> stage failed, run failed
  with REPORT_MISSING
- Agent binding (doc 3.3): fixed instance (must declare report.generate)
  or capability-tag matching; none -> NO_ELIGIBLE_AGENT
- Cancel (doc 5.2): run canceled, dispatch stops at stage boundary
- Boot recovery (doc 7.2): running runs marked failed HUB_RESTART_INTERRUPTED
- Retention (doc 7.1): 90 day sweep on the shared 24h ticker
- panel.workflows.*: list/get/save/start/runs/run/cancel/delete
- Router: task.completed/task.failed advance/terminate the owning run
- Audit: workflow.started/completed/failed/canceled (doc 7.3)

## Verification

- gofmt: zero diff
- go vet: all hub modules green
- T1-T12 (cmd/selftest): fails=0
- S0-S5 (cmd/selftest2): fails=0
- P3-01..P3-08 (cmd/selftest3): fails=0
- W1-W5 (cmd/selftest4): fails=0
  W1 workflow saved / W2 registered+started / W3 stage 0 dispatched /
  W4 handoff format / W5 REPORT_MISSING fails the run
- GOOS=windows cross-compile: green

## Not delivered (documented deferral)

- Workflow editor canvas + sidebar form + run view (chapter 9 gate)
- DAG / parallel stages (out of scope by design)

## Delivered post-acceptance (2026-11-15)

- Chain import/export JSON (doc 1.3), format `chain.v1` (definitions only:
  name/description/stages; no ids, status, timestamps or run history):
  - workflows/chainio.go: ChainFile + ChainFileStage + ChainFileVersion
  - panel.workflows.export / panel.workflows.import; import creates a new
    workflow id with draft status; BAD_CHAIN on bad version/validation
- selftest4 W6-W8: export -> import -> roundtrip definition preserved
- Regression: selftest (T1-T12) fails=0; gofmt/go vet clean
- GOOS=windows cross-compile green for hub, harness-sdk and plugin-sdk
