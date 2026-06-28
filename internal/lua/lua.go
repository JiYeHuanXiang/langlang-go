// Package lua 提供 Lua 脚本引擎封装，作为 RedLang 的并行可选引擎。
// 使用 gopher-lua 实现，纯 Go，无需 CGo。
package lua

import (
	"errors"
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/super1207/langlang-go/internal/redlang"
)

// ValidateLua 校验 Lua 代码的语法，不执行。
func ValidateLua(code string) error {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	// LoadString 只做解析和编译，不执行
	_, err := L.LoadString(code)
	if err != nil {
		return fmt.Errorf("lua 语法错误: %w", err)
	}
	return nil
}

// EvalLua 执行 Lua 脚本，并返回输出。
// ctx 提供 BotAPI / LogFunc / GetVar / SetVar 等绑定。
func EvalLua(code string, ctx *redlang.AppContext) (string, error) {
	L := lua.NewState()
	defer L.Close()

	// 打开基础库（math, string, table 等），方便脚本使用
	// 但不打开 io / os 模块，防止文件系统访问
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}); err != nil {
			return "", fmt.Errorf("加载库 %s 失败: %w", pair.n, err)
		}
	}

	// 注入 bot_api 函数：调用机器人 API
	L.SetGlobal("bot_api", L.NewFunction(func(L *lua.LState) int {
		platform := L.CheckString(1)
		selfID := L.CheckString(2)
		action := L.CheckString(3)
		paramsTable := L.CheckTable(4)

		params := make(map[string]any)
		paramsTable.ForEach(func(k, v lua.LValue) {
			key := lua.LVAsString(k)
			switch val := v.(type) {
			case lua.LString:
				params[key] = string(val)
			case lua.LNumber:
				params[key] = float64(val)
			case lua.LBool:
				params[key] = bool(val)
			default:
				params[key] = val.String()
			}
		})

		if ctx != nil && ctx.BotAPI != nil {
			result, err := ctx.BotAPI(platform, selfID, action, params)
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			// 将 map 转为 Lua table
			tbl := L.NewTable()
			for k, v := range result {
				tbl.RawSetString(k, toLuaValue(L, v))
			}
			L.Push(tbl)
			return 1
		}
		L.Push(lua.LNil)
		L.Push(lua.LString("BotAPI not available"))
		return 2
	}))

	// 注入 log 函数：写入日志
	L.SetGlobal("log", L.NewFunction(func(L *lua.LState) int {
		level := L.CheckString(1)
		msg := L.CheckString(2)
		if ctx != nil && ctx.LogFunc != nil {
			ctx.LogFunc(level, msg)
		}
		return 0
	}))

	// 注入 get_var 函数：读取全局变量
	L.SetGlobal("get_var", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		if ctx != nil && ctx.GetVar != nil {
			L.Push(lua.LString(ctx.GetVar(key)))
		} else {
			L.Push(lua.LString(""))
		}
		return 1
	}))

	// 注入 set_var 函数：设置全局变量
	L.SetGlobal("set_var", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		value := L.CheckString(2)
		if ctx != nil && ctx.SetVar != nil {
			ctx.SetVar(key, value)
		}
		return 0
	}))

	// 注入 send_msg 便捷函数（简化调用）
	L.SetGlobal("send_msg", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		// 使用默认平台/ID 的空调用
		if ctx != nil && ctx.BotAPI != nil {
			_, err := ctx.BotAPI("", "", "send_msg", map[string]any{
				"message": msg,
			})
			if err != nil {
				L.Push(lua.LBool(false))
				return 1
			}
		}
		L.Push(lua.LBool(true))
		return 1
	}))

	// 捕获脚本的输出
	var output strings.Builder
	// 重定向 print 到我们的 buffer
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		args := make([]string, 0, top)
		for i := 1; i <= top; i++ {
			args = append(args, L.Get(i).String())
		}
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(strings.Join(args, "\t"))
		return 0
	}))

	// 执行脚本
	if err := L.DoString(code); err != nil {
		return "", fmt.Errorf("lua 执行错误: %w", err)
	}

	return output.String(), nil
}

// toLuaValue 将 Go any 转为 Lua LValue
func toLuaValue(L *lua.LState, v any) lua.LValue {
	switch val := v.(type) {
	case string:
		return lua.LString(val)
	case float64:
		return lua.LNumber(val)
	case int:
		return lua.LNumber(val)
	case bool:
		return lua.LBool(val)
	case nil:
		return lua.LNil
	case map[string]any:
		tbl := L.NewTable()
		for k, v2 := range val {
			tbl.RawSetString(k, toLuaValue(L, v2))
		}
		return tbl
	case []any:
		tbl := L.NewTable()
		for i, v2 := range val {
			tbl.RawSetInt(i+1, toLuaValue(L, v2))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

// Ensure LuaError 等不被编译器优化掉
var _ = errors.New
