package workflows

import "errors"

// Engine errors.
var (
	ErrNoStages        = errors.New("workflow has no stages")
	ErrNoEligibleAgent = errors.New("no eligible agent for stage")
	ErrRunNotRunning   = errors.New("run is not running")
)

// ErrRunNotFound means no running chain owns the given task.
var ErrRunNotFound = errors.New("no running workflow owns the task")
