package models

type BadRequestError struct {
	Message string
}

func (e BadRequestError) Error() string { return e.Message }

type PermissionError struct {
	Message string
}

func (e PermissionError) Error() string { return e.Message }

type NotFoundError struct {
	Message string
}

func (e NotFoundError) Error() string { return e.Message }

type ConflictError struct {
	Message string
}

func (e ConflictError) Error() string { return e.Message }

type ServerFaultError struct {
	Message string
}

func (e ServerFaultError) Error() string { return e.Message }

type UnprocessableEntityError struct {
	Message string
}

func (e UnprocessableEntityError) Error() string { return e.Message }
