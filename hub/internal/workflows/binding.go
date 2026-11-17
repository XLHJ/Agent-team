package workflows

// SetBinding installs the runtime agent-selection callbacks.
func (e *Engine) SetBinding(el Eligible, sel Selector) {
	e.eligible = el
	e.pick = sel
}

// SetDispatcher installs the task dispatcher used for stage dispatch.
func (e *Engine) SetDispatcher(d Dispatcher) {
	e.disp = d
}
