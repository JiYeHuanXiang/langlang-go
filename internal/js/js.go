// Package js 提供 JavaScript 脚本引擎封装，作为 RedLang / Lua 的并行可选引擎。
// 使用 goja 实现，纯 Go，无需 CGo。
package js

import (
	"fmt"
	"strings"

	"github.com/dop251/goja"

	"github.com/jiyehuanxiang/langlang-go/internal/redlang"
)

// ValidateJS 校验 JavaScript 代码的语法，不执行。
func ValidateJS(code string) error {
	_, err := goja.Compile("script.js", code, true)
	if err != nil {
		return fmt.Errorf("javascript 语法错误: %w", err)
	}
	return nil
}

// EvalJS 执行 JavaScript 脚本，并返回输出。
// ctx 提供 BotAPI / LogFunc / GetVar / SetVar 等绑定。
func EvalJS(code string, ctx *redlang.AppContext) (string, error) {
	vm := goja.New()

	// 捕获脚本的输出
	var output strings.Builder

	// 注入 console.log / console.warn / console.error / console.info
	consoleObj := vm.NewObject()
	consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		writeArgs(&output, call.Arguments)
		return goja.Undefined()
	})
	consoleObj.Set("info", func(call goja.FunctionCall) goja.Value {
		writeArgs(&output, call.Arguments)
		return goja.Undefined()
	})
	consoleObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		writeArgs(&output, call.Arguments)
		return goja.Undefined()
	})
	consoleObj.Set("error", func(call goja.FunctionCall) goja.Value {
		writeArgs(&output, call.Arguments)
		return goja.Undefined()
	})
	vm.Set("console", consoleObj)

	// 注入 log 函数：写入日志（与 Lua 一致的接口）
	vm.Set("log", func(call goja.FunctionCall) goja.Value {
		level := call.Argument(0).String()
		msg := call.Argument(1).String()
		if ctx != nil && ctx.LogFunc != nil {
			ctx.LogFunc(level, msg)
		}
		return goja.Undefined()
	})

	// 注入 bot_api 函数：调用机器人 API
	vm.Set("bot_api", func(call goja.FunctionCall) goja.Value {
		platform := call.Argument(0).String()
		selfID := call.Argument(1).String()
		action := call.Argument(2).String()
		paramsObj := call.Argument(3).ToObject(vm)

		params := make(map[string]any)
		for _, key := range paramsObj.Keys() {
			val := paramsObj.Get(key)
			params[key] = val.Export()
		}

		if ctx != nil && ctx.BotAPI != nil {
			result, err := ctx.BotAPI(platform, selfID, action, params)
			if err != nil {
				// 返回 {error: "..."} 对象
				errObj := vm.NewObject()
				errObj.Set("error", err.Error())
				return errObj
			}
			return vm.ToValue(result)
		}
		errObj := vm.NewObject()
		errObj.Set("error", "BotAPI not available")
		return errObj
	})

	// 注入 get_var 函数：读取全局变量
	vm.Set("get_var", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		if ctx != nil && ctx.GetVar != nil {
			return vm.ToValue(ctx.GetVar(key))
		}
		return vm.ToValue("")
	})

	// 注入 set_var 函数：设置全局变量
	vm.Set("set_var", func(call goja.FunctionCall) goja.Value {
		key := call.Argument(0).String()
		value := call.Argument(1).String()
		if ctx != nil && ctx.SetVar != nil {
			ctx.SetVar(key, value)
		}
		return goja.Undefined()
	})

	// 注入 send_msg 便捷函数
	vm.Set("send_msg", func(call goja.FunctionCall) goja.Value {
		msg := call.Argument(0).String()
		if ctx != nil && ctx.BotAPI != nil {
			_, err := ctx.BotAPI("", "", "send_msg", map[string]any{
				"message": msg,
			})
			if err != nil {
				return vm.ToValue(false)
			}
		}
		return vm.ToValue(true)
	})

	// 执行脚本
	compiled, err := goja.Compile("script.js", code, true)
	if err != nil {
		return "", fmt.Errorf("javascript 语法错误: %w", err)
	}
	_, err = vm.RunProgram(compiled)
	if err != nil {
		return "", fmt.Errorf("javascript 执行错误: %w", err)
	}

	return output.String(), nil
}

// writeArgs 将函数调用的参数列表写入 output，参数间以空格分隔。
func writeArgs(output *strings.Builder, args []goja.Value) {
	if output.Len() > 0 {
		output.WriteString("\n")
	}
	for i, arg := range args {
		if i > 0 {
			output.WriteString(" ")
		}
		output.WriteString(arg.String())
	}
}
