package service

// Result 是所有 Wails 绑定方法的统一返回结构
type Result[T any] struct {
	Ok    bool   `json:"ok"`
	Data  T      `json:"data"`
	Error string `json:"error,omitempty"`
}

func Ok[T any](data T) Result[T]      { return Result[T]{Ok: true, Data: data} }
func Err[T any](msg string) Result[T] { return Result[T]{Ok: false, Error: msg} }
