package bot

import "errors"

// 通用错误定义
var (
	ErrAdapterNotFound    = errors.New("适配器未找到")
	ErrPlatformNotSupport = errors.New("不支持的平台")
	ErrActionNotSupport   = errors.New("不支持的动作")
	ErrConnectionFailed   = errors.New("连接失败")
	ErrNotImplemented     = errors.New("未实现")
)
