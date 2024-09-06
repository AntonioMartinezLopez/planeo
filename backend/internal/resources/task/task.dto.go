package task

type TaskOutput struct {
	Body struct {
		Message string `json:"message" example:"Hello, world!" doc:"Greeting message"`
	}
}

type GetTaskInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	GroupId      string `path:"groupId" doc:"ID of the group to which the given task belongs"`
	TaskId       string `path:"taskId" doc:"ID of the task"`
}

type GetTasksInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	GroupId      string `path:"groupId" doc:"ID of the group to which the given task belongs"`
}

type CreateTaskInput struct {
	Organization string `path:"organization" doc:"ID of the organization"`
	GroupId      string `path:"groupId" doc:"ID of the group to which the given task belongs"`
}

type UpdateTaskInput struct {
	GetTaskInput
}

type DeleteTaskInput struct {
	GetTaskInput
}
