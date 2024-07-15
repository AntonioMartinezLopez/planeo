package handler

import "net/http"

type TaskHandler struct{}

func NewTaskHandler() *TaskHandler {
	return &TaskHandler{}
}

func (s *TaskHandler) GetTask(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("GetTask endpoint"))
}

func (s *TaskHandler) CreateTask(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("CreateTask endpoint"))
}

func (s *TaskHandler) UpdateTask(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("UpdateTask endpoint"))
}

func (s *TaskHandler) DeleteTask(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("DeleteTask endpoint"))
}
