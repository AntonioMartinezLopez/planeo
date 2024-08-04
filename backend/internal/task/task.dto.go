package task

type TaskOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetTaskInput struct {
	GroupId string `path:"groupId" doc:"ID of the group to which the given task belongs"`
	TaskId  string `path:"taskId" doc:"ID of the task"`
}

type GetTasksInput struct {
	GroupId string `path:"groupId" doc:"ID of the group to which the given task belongs"`
}

type CreateTaskInput struct {
	GroupId string `path:"groupId" doc:"ID of the group to which the given task belongs"`
}

type UpdateTaskInput struct {
	GetTaskInput
}

type DeleteTaskInput struct {
	GetTaskInput
}
