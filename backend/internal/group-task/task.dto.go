package task

type TaskOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetTaskInput struct {
	GetGroupInput
	TaskId string `path:"taskId" doc:"ID of the task"`
}

type CreateTaskInput struct{}
type UpdateTaskInput struct {
	GetTaskInput
}
type DeleteTaskInput struct {
	GetTaskInput
}
