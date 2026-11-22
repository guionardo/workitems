package workitem

// Status is the normalized lifecycle category of a task.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in-progress"
	StatusDone       Status = "done"
	StatusUnknown    Status = "unknown"
)
