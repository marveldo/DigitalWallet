package shared
import "fmt"


type AppError struct {
	Context string
	Code    int
	Err  error
	Message string
}

func (e *AppError) Error() string {
	msg := "App Error"
	err_msg := e.Message
	ctx_msg := e.Context
	status_code := e.Code
	if ctx_msg != "" {
		msg = fmt.Sprintf("%s: context: %s", msg, ctx_msg)
	}
	if err_msg != "" {
		msg = fmt.Sprintf("%s: msg: %s", msg, err_msg)
	}
	if status_code != 0 {
		msg = fmt.Sprintf("%s: status code: %v", msg, status_code)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: err: %v", msg, e.Err)
	}
	return msg //fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}