package redlang

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jiyehuanxiang/langlang-go/internal/log"
)

// Scope 是 RedLang 的变量作用域
type Scope struct {
	parent *Scope
	vars   map[string]*RedValue
}

// NewScope 创建新作用域
func NewScope() *Scope {
	return &Scope{
		vars: make(map[string]*RedValue),
	}
}

// NewChildScope 创建子作用域（继承父作用域）
func NewChildScope(parent *Scope) *Scope {
	return &Scope{
		parent: parent,
		vars:   make(map[string]*RedValue),
	}
}

// Get 获取变量
func (s *Scope) Get(name string) (*RedValue, bool) {
	v, ok := s.vars[name]
	if ok {
		return v, true
	}
	if s.parent != nil {
		return s.parent.Get(name)
	}
	return nil, false
}

// Set 设置变量（仅在当前作用域）
func (s *Scope) Set(name string, v *RedValue) {
	s.vars[name] = v
}

// SetClosest 沿作用域链查找并设置变量，若都不存在则在当前作用域定义
func (s *Scope) SetClosest(name string, v *RedValue) {
	if s.HasLocal(name) {
		s.vars[name] = v
		return
	}
	if s.parent != nil {
		s.parent.SetClosest(name, v)
		return
	}
	// 不在任何作用域，在当前定义
	s.vars[name] = v
}

// HasLocal 检查变量是否在当前作用域（不向上查找）
func (s *Scope) HasLocal(name string) bool {
	_, ok := s.vars[name]
	return ok
}

// Has 检查变量是否存在
func (s *Scope) Has(name string) bool {
	_, ok := s.Get(name)
	return ok
}

// LoopSignal 循环控制信号
type LoopSignal int

const (
	LoopNone     LoopSignal = iota
	LoopBreak                // 跳出
	LoopContinue             // 继续
)

// ReturnSignal 返回控制信号
type ReturnSignal int

const (
	ReturnNone ReturnSignal = iota
	ReturnNow               // 返回
)

// Runtime 是 RedLang 脚本运行时
type Runtime struct {
	Scope       *Scope
	Globals     *Scope
	Functions   map[string]*BuiltinFunc
	loopControl LoopSignal
	retSignal   ReturnSignal
}

// BuiltinFunc 是内置函数定义
type BuiltinFunc struct {
	Name string
	Fn   func(args []*RedValue, rt *Runtime) (*RedValue, error)
}

// NewRuntime 创建新运行时
func NewRuntime() *Runtime {
	rt := &Runtime{
		Scope:     NewScope(),
		Globals:   NewScope(),
		Functions: make(map[string]*BuiltinFunc),
	}
	rt.registerBuiltins()
	return rt
}

// Eval 求值一段 AST，返回值
func (rt *Runtime) Eval(ast Ast) (*RedValue, error) {
	var parts []string
	i := 0
	for i < len(ast) {
		if rt.retSignal == ReturnNow {
			break
		}

		node := ast[i]

		// 处理 如果…则…否则 结构
		if node.Type == TypeCommand && node.Cmd.Name == "如果" {
			condResult, err := rt.EvalNode(node)
			if err != nil {
				return nil, err
			}
			isTrue := condResult.IsTrue()
			i++

			// 消费 【则】 分支
			if i < len(ast) && ast[i].Type == TypeCommand && ast[i].Cmd.Name == "则" {
				if isTrue {
					val, err := rt.EvalNode(ast[i])
					if err != nil {
						return nil, err
					}
					if val.Type != ValNull {
						parts = append(parts, val.String())
					}
				}
				i++

				// 消费 【否则】 分支
				if i < len(ast) && ast[i].Type == TypeCommand && ast[i].Cmd.Name == "否则" {
					if !isTrue {
						val, err := rt.EvalNode(ast[i])
						if err != nil {
							return nil, err
						}
						if val.Type != ValNull {
							parts = append(parts, val.String())
						}
					}
					i++
				}
			}
			continue
		}

		// 处理 计次循环…计次循环尾 结构
		if node.Type == TypeCommand && node.Cmd.Name == "计次循环" {
			countVal, err := rt.EvalNode(node)
			if err != nil {
				return nil, err
			}
			count, _ := strconv.Atoi(countVal.String())
			i++

			// 收集循环体节点直到 计次循环尾
			bodyStart := i
			bodyEnd := len(ast)
			for j := i; j < len(ast); j++ {
				if ast[j].Type == TypeCommand && ast[j].Cmd.Name == "计次循环尾" {
					bodyEnd = j
					break
				}
			}
			body := ast[bodyStart:bodyEnd]

			// 执行循环
			rt.loopControl = LoopNone
			for n := 0; n < count; n++ {
				if rt.loopControl == LoopBreak {
					break
				}
				// 设置迭代变量
				rt.Scope.Set("循环次数", NewText(strconv.Itoa(n+1)))
				// 执行循环体
			bodyLoop:
				for _, bodyNode := range body {
					val, bodyErr := rt.EvalNode(bodyNode)
					if bodyErr != nil {
						return nil, bodyErr
					}
					if val.Type != ValNull {
						parts = append(parts, val.String())
					}
					switch rt.loopControl {
					case LoopBreak:
						break bodyLoop
					case LoopContinue:
						rt.loopControl = LoopNone
						break bodyLoop
					}
				}
			}
			rt.loopControl = LoopNone
			i = bodyEnd + 1 // 跳过 计次循环尾
			continue
		}

		// 处理 循环…循环尾 结构（条件循环）
		if node.Type == TypeCommand && node.Cmd.Name == "循环" {
			condVal, err := rt.EvalNode(node)
			if err != nil {
				return nil, err
			}
			i++

			// 收集循环体节点直到 循环尾
			bodyStart := i
			bodyEnd := len(ast)
			for j := i; j < len(ast); j++ {
				if ast[j].Type == TypeCommand && ast[j].Cmd.Name == "循环尾" {
					bodyEnd = j
					break
				}
			}
			body := ast[bodyStart:bodyEnd]

			// 执行条件循环
			rt.loopControl = LoopNone
			for condVal.IsTrue() {
				if rt.loopControl == LoopBreak {
					break
				}
			bodyWhileLoop:
				for _, bodyNode := range body {
					val, bodyErr := rt.EvalNode(bodyNode)
					if bodyErr != nil {
						return nil, bodyErr
					}
					if val.Type != ValNull {
						parts = append(parts, val.String())
					}
					switch rt.loopControl {
					case LoopBreak:
						break bodyWhileLoop
					case LoopContinue:
						rt.loopControl = LoopNone
						break bodyWhileLoop
					}
				}
				if rt.loopControl == LoopBreak {
					rt.loopControl = LoopNone
					break
				}
				// 重新求值条件（条件表达式可能依赖变量变化）
				nextCond, condErr := rt.EvalNode(node)
				if condErr != nil {
					return nil, condErr
				}
				condVal = nextCond
			}
			rt.loopControl = LoopNone
			i = bodyEnd + 1
			continue
		}

		val, err := rt.EvalNode(node)
		if err != nil {
			return nil, err
		}
		if val.Type != ValNull {
			parts = append(parts, val.String())
		}
		i++
	}
	if len(parts) == 0 {
		return NewText(""), nil
	}
	if len(parts) == 1 {
		return NewText(parts[0]), nil
	}
	return NewText(strings.Join(parts, "")), nil
}

// EvalNode 求值单个 AST 节点
func (rt *Runtime) EvalNode(node AstNode) (*RedValue, error) {
	switch node.Type {
	case TypeText:
		return NewText(node.Text), nil
	case TypeCommand:
		return rt.callFunc(node.Cmd)
	default:
		return NewNull(), nil
	}
}

// callFunc 调用命令
func (rt *Runtime) callFunc(cmd *AstCmd) (*RedValue, error) {
	// 特殊处理：函数定义 — 从 RawBody 创建函数值
	if cmd.Name == "函数定义" && cmd.RawBody != "" {
		bodyAst, err := Parse(cmd.RawBody)
		if err != nil {
			return NewNull(), nil
		}
		return NewFun(&bodyAst), nil
	}

	// 先求值所有参数（用 evalArg 以保留值类型，不被文本化）
	var evaluatedArgs []*RedValue
	for _, argAst := range cmd.Args {
		val, err := rt.evalArg(argAst)
		if err != nil {
			return nil, fmt.Errorf("求值参数失败 %s: %w", cmd.Name, err)
		}
		evaluatedArgs = append(evaluatedArgs, val)
	}

	// 查找内置函数
	if fn, ok := rt.Functions[cmd.Name]; ok {
		return fn.Fn(evaluatedArgs, rt)
	}

	// 查找变量（可能存了函数）
	if val, ok := rt.Scope.Get(cmd.Name); ok && val.Type == ValFun {
		return rt.callUserFunc(val, evaluatedArgs)
	}

	// 未知命令 — 输出原始文字
	return NewText(fmt.Sprintf("【%s】", cmd.Name)), nil
}

// evalArg 求值参数，保留值类型（不将非空值文本化）
// 参数 ast 通常包含一个节点（文本或命令），但也可以有多个文本+命令混合
func (rt *Runtime) evalArg(ast Ast) (*RedValue, error) {
	var lastVal *RedValue
	for _, node := range ast {
		val, err := rt.EvalNode(node)
		if err != nil {
			return nil, err
		}
		if val.Type != ValNull {
			lastVal = val
		}
	}
	if lastVal != nil {
		return lastVal, nil
	}
	// 如果全部为空，将文本部分拼接返回
	var parts []string
	for _, node := range ast {
		if node.Type == TypeText {
			parts = append(parts, node.Text)
		}
	}
	if len(parts) > 0 {
		return NewText(strings.Join(parts, "")), nil
	}
	return NewText(""), nil
}

// callUserFunc 调用用户定义的函数
func (rt *Runtime) callUserFunc(val *RedValue, args []*RedValue) (*RedValue, error) {
	if val.Fun == nil {
		return NewNull(), nil
	}
	// 创建子作用域，将参数注入为 参数1..参数N
	childScope := NewChildScope(rt.Scope)
	for i, arg := range args {
		childScope.Set(fmt.Sprintf("参数%d", i+1), arg)
	}
	childScope.Set("参数个数", NewText(fmt.Sprintf("%d", len(args))))
	// 同时支持按名称绑定（如果函数定义时指定了参数名，后续可扩展）
	oldScope := rt.Scope
	oldRetSignal := rt.retSignal
	oldLoopControl := rt.loopControl
	rt.Scope = childScope
	rt.retSignal = ReturnNone
	rt.loopControl = LoopNone
	result, err := rt.Eval(*val.Fun)
	rt.Scope = oldScope
	rt.retSignal = oldRetSignal
	rt.loopControl = oldLoopControl
	return result, err
}

// EvalScript 求值完整脚本字符串
func (rt *Runtime) EvalScript(input string) (*RedValue, error) {
	ast, err := Parse(input)
	if err != nil {
		return nil, fmt.Errorf("解析失败: %w", err)
	}
	return rt.Eval(ast)
}

// RegisterFunc 注册内置函数
func (rt *Runtime) RegisterFunc(name string, fn func(args []*RedValue, rt *Runtime) (*RedValue, error)) {
	rt.Functions[name] = &BuiltinFunc{
		Name: name,
		Fn:   fn,
	}
}

// getStack 获取当前作用域的栈
func getStack(rt *Runtime) *[]*RedValue {
	stackVal, ok := rt.Scope.Get("__stack__")
	if !ok {
		stack := make([]*RedValue, 0)
		stackVal = NewArray(stack)
		rt.Scope.Set("__stack__", stackVal)
	}
	return &stackVal.Array
}

// strsToVals 将字符串切片转为 RedValue 数组
func strsToVals(strs []string) []*RedValue {
	vals := make([]*RedValue, len(strs))
	for i, s := range strs {
		vals[i] = NewText(s)
	}
	return vals
}

// jsonToRedValue 将任意 Go 类型转为 RedValue
func jsonToRedValue(v any) *RedValue {
	if v == nil {
		return NewNull()
	}
	switch val := v.(type) {
	case string:
		return NewText(val)
	case float64:
		return NewText(strconv.FormatFloat(val, 'f', -1, 64))
	case bool:
		return NewBool(val)
	case []any:
		items := make([]*RedValue, len(val))
		for i, item := range val {
			items[i] = jsonToRedValue(item)
		}
		return NewArray(items)
	case map[string]any:
		obj := make(map[string]*RedValue, len(val))
		for k, v2 := range val {
			obj[k] = jsonToRedValue(v2)
		}
		return NewObject(obj)
	default:
		return NewText(fmt.Sprintf("%v", v))
	}
}

// isMaster 检查指定用户是否是主人
func isMaster(userID string, rt *Runtime) bool {
	if masters, ok := rt.Scope.Get("__masters__"); ok && masters.Type == ValArray {
		for _, m := range masters.Array {
			if m.String() == userID {
				return true
			}
		}
	}
	return false
}

// registerBuiltins 注册内置函数
func (rt *Runtime) registerBuiltins() {
	// 文本输出
	rt.RegisterFunc("输出", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 {
			return NewText(""), nil
		}
		return args[0], nil
	})

	// 如果 — 条件判断
	// 支持两种调用模式：
	//   1) 分组模式：如果(条件) — 返回条件结果供 Eval 的 如果…则…否则 分组逻辑使用
	//   2) 内联模式：如果(条件, 则分支, [否则分支]) — 直接返回结果
	rt.RegisterFunc("如果", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		if len(args) == 1 {
			// 分组模式：只传了条件，返回条件值本身供 Eval 分组判断
			return args[0], nil
		}
		// 内联模式：如果(条件, 则分支, [否则分支])
		if args[0].IsTrue() {
			return args[1], nil
		}
		if len(args) >= 3 {
			return args[2], nil
		}
		return NewNull(), nil
	})

	// 且
	rt.RegisterFunc("且", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		for _, arg := range args {
			if !arg.IsTrue() {
				return NewBool(false), nil
			}
		}
		return NewBool(true), nil
	})

	// 或
	rt.RegisterFunc("或", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		for _, arg := range args {
			if arg.IsTrue() {
				return NewBool(true), nil
			}
		}
		return NewBool(false), nil
	})

	// == 等于
	rt.RegisterFunc("==", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewBool(false), nil
		}
		return NewBool(args[0].String() == args[1].String()), nil
	})

	// != 不等于
	rt.RegisterFunc("!=", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewBool(true), nil
		}
		return NewBool(args[0].String() != args[1].String()), nil
	})

	// > 大于
	rt.RegisterFunc(">", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewBool(false), nil
		}
		return NewBool(args[0].String() > args[1].String()), nil
	})

	// < 小于
	rt.RegisterFunc("<", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewBool(false), nil
		}
		return NewBool(args[0].String() < args[1].String()), nil
	})

	// >= 大于等于
	rt.RegisterFunc(">=", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewBool(false), nil
		}
		return NewBool(args[0].String() >= args[1].String()), nil
	})

	// <= 小于等于
	rt.RegisterFunc("<=", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewBool(false), nil
		}
		return NewBool(args[0].String() <= args[1].String()), nil
	})

	// 加
	rt.RegisterFunc("加", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText("0"), nil
		}
		a, _ := strconv.ParseFloat(args[0].String(), 64)
		b, _ := strconv.ParseFloat(args[1].String(), 64)
		return NewText(fmt.Sprintf("%g", a+b)), nil
	})

	// 减
	rt.RegisterFunc("减", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText("0"), nil
		}
		a, _ := strconv.ParseFloat(args[0].String(), 64)
		b, _ := strconv.ParseFloat(args[1].String(), 64)
		return NewText(fmt.Sprintf("%g", a-b)), nil
	})

	// 乘
	rt.RegisterFunc("乘", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText("0"), nil
		}
		a, _ := strconv.ParseFloat(args[0].String(), 64)
		b, _ := strconv.ParseFloat(args[1].String(), 64)
		return NewText(fmt.Sprintf("%g", a*b)), nil
	})

	// 除
	rt.RegisterFunc("除", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText("0"), nil
		}
		a, _ := strconv.ParseFloat(args[0].String(), 64)
		b, _ := strconv.ParseFloat(args[1].String(), 64)
		if b == 0 {
			return NewText("0"), nil
		}
		return NewText(fmt.Sprintf("%g", a/b)), nil
	})

	// 模
	rt.RegisterFunc("模", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText("0"), nil
		}
		a, _ := strconv.ParseInt(args[0].String(), 10, 64)
		b, _ := strconv.ParseInt(args[1].String(), 10, 64)
		if b == 0 {
			return NewText("0"), nil
		}
		return NewText(fmt.Sprintf("%d", a%b)), nil
	})

	// 取文本长度
	rt.RegisterFunc("取文本长度", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 {
			return NewText("0"), nil
		}
		return NewText(fmt.Sprintf("%d", len(args[0].String()))), nil
	})

	// 取数组长度
	rt.RegisterFunc("取数组长度", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 || args[0].Type != ValArray {
			return NewText("0"), nil
		}
		return NewText(fmt.Sprintf("%d", len(args[0].Array))), nil
	})

	// 取数组成员
	rt.RegisterFunc("取数组成员", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 || args[0].Type != ValArray {
			return NewNull(), nil
		}
		idx := 0
		if _, err := fmt.Sscanf(args[1].String(), "%d", &idx); err != nil {
			return NewNull(), nil
		}
		if idx < 0 || idx >= len(args[0].Array) {
			return NewNull(), nil
		}
		return args[0].Array[idx], nil
	})

	// 寻找文本
	rt.RegisterFunc("寻找文本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText("-1"), nil
		}
		idx := strings.Index(args[0].String(), args[1].String())
		return NewText(fmt.Sprintf("%d", idx)), nil
	})

	// 到文本
	rt.RegisterFunc("到文本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 {
			return NewText(""), nil
		}
		return NewText(args[0].String()), nil
	})

	// 合并文本
	rt.RegisterFunc("合并文本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		var parts []string
		for _, arg := range args {
			parts = append(parts, arg.String())
		}
		return NewText(strings.Join(parts, "")), nil
	})

	// 子文本替换
	rt.RegisterFunc("子文本替换", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 3 {
			return NewText(""), nil
		}
		return NewText(strings.ReplaceAll(args[0].String(), args[1].String(), args[2].String())), nil
	})

	// 令 / 赋值 — 将值存入变量（语句级，返回空值避免输出回显）
	rt.RegisterFunc("令", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		rt.Scope.Set(name, args[1])
		return NewNull(), nil
	})
	rt.RegisterFunc("赋值", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		rt.Scope.Set(name, args[1])
		return NewNull(), nil
	})

	// 定义变量 — 在当前作用域定义变量（若已存在则替换）
	rt.RegisterFunc("定义变量", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		rt.Scope.Set(name, args[1])
		return NewNull(), nil
	})

	// 赋值变量 — 沿作用域链修改最近变量，若都不存在则在当前作用域定义
	rt.RegisterFunc("赋值变量", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		rt.Scope.SetClosest(name, args[1])
		return NewNull(), nil
	})

	// 取 / 读取 — 从变量读取值
	rt.RegisterFunc("取", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		name := args[0].String()
		if val, ok := rt.Scope.Get(name); ok {
			return val, nil
		}
		return NewText(""), nil
	})
	rt.RegisterFunc("读取", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		name := args[0].String()
		if val, ok := rt.Scope.Get(name); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 变量 — 沿作用域链读取最近变量（与 【取】 语义相同）
	rt.RegisterFunc("变量", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		name := args[0].String()
		if val, ok := rt.Scope.Get(name); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 计次循环 — 执行 N 次循环体（与 Eval 分组逻辑配合）
	rt.RegisterFunc("计次循环", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		return args[0], nil
	})

	// 计次循环尾 — 循环体结束标记（Eval 层处理，返回空）
	rt.RegisterFunc("计次循环尾", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewNull(), nil
	})

	// 循环 — 条件循环（与 Eval 分组逻辑配合）
	rt.RegisterFunc("循环", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		return args[0], nil
	})

	// 循环尾 — 条件循环尾标记
	rt.RegisterFunc("循环尾", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewNull(), nil
	})

	// 跳出 — 跳出当前循环
	rt.RegisterFunc("跳出", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		rt.loopControl = LoopBreak
		return NewNull(), nil
	})

	// 继续 — 跳到下一次迭代
	rt.RegisterFunc("继续", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		rt.loopControl = LoopContinue
		return NewNull(), nil
	})

	// 则 — 如果条件分支中，条件为真时返回参数
	rt.RegisterFunc("则", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 {
			return NewNull(), nil
		}
		return args[0], nil
	})

	// 否则 — 如果条件分支中，条件为假时返回参数
	rt.RegisterFunc("否则", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 {
			return NewNull(), nil
		}
		return args[0], nil
	})

	// 写日志 — 接入正式日志系统
	rt.RegisterFunc("写日志", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) > 0 {
			log.Info("[RedLang]", "msg", args[0].String())
		}
		return NewNull(), nil
	})

	// 计算 — 表达式求值
	rt.RegisterFunc("计算", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		expr := args[0].String()
		result, err := EvalExpr(expr, func(name string) string {
			if val, ok := rt.Scope.Get(name); ok {
				return val.String()
			}
			return ""
		})
		if err != nil {
			return NewText(""), nil
		}
		return NewText(result), nil
	})

	// 函数定义 — 定义一个函数
	rt.RegisterFunc("函数定义", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 函数体通过 RawBody 传递，在 callFunc 中特殊处理
		return NewNull(), nil
	})

	// 调用函数 — 调用一个函数
	rt.RegisterFunc("调用函数", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		fnVal := args[0]
		callArgs := args[1:]
		if fnVal.Type == ValFun {
			return rt.callUserFunc(fnVal, callArgs)
		}
		// 也支持函数名（文本）
		if fnVal.Type == ValText {
			if val, ok := rt.Scope.Get(fnVal.Text); ok && val.Type == ValFun {
				return rt.callUserFunc(val, callArgs)
			}
		}
		return NewNull(), nil
	})

	// 参数 — 获取第 N 个参数
	rt.RegisterFunc("参数", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		idxStr := args[0].String()
		idx := 0
		// 解析序号（1-based）
		if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil || idx < 1 {
			return NewText(""), nil
		}
		// 尝试从当前作用域获取 参数N
		if val, ok := rt.Scope.Get(fmt.Sprintf("参数%d", idx)); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 参数个数 — 获取当前函数调用的参数个数
	rt.RegisterFunc("参数个数", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 从作用域中查找参数个数变量
		if val, ok := rt.Scope.Get("参数个数"); ok {
			return val, nil
		}
		return NewText("0"), nil
	})

	// 返回 — 跳出当前作用域
	rt.RegisterFunc("返回", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		rt.retSignal = ReturnNow
		return NewNull(), nil
	})

	// 定义命令 — 全局注册自定义命令（重启前有效）
	rt.RegisterFunc("定义命令", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		cmdName := args[0].String()
		cmdBody := args[1]
		if cmdBody.Type == ValFun {
			// 将函数内容注册为命令
			rt.RegisterFunc(cmdName, func(callArgs []*RedValue, rtt *Runtime) (*RedValue, error) {
				return rtt.callUserFunc(cmdBody, callArgs)
			})
		}
		return NewNull(), nil
	})

	// 定义二类命令 — 参数不提前求值（注册一个 RawBody 命令）
	rt.RegisterFunc("定义二类命令", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		cmdName := args[0].String()
		cmdBody := args[1]
		if cmdBody.Type == ValFun {
			// 注册为命令，但参数不提前求值
			rt.RegisterFunc(cmdName, func(callArgs []*RedValue, rtt *Runtime) (*RedValue, error) {
				return rtt.callUserFunc(cmdBody, callArgs)
			})
		}
		return NewNull(), nil
	})

	// 二类参数 — 在二类命令中获取参数
	rt.RegisterFunc("二类参数", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		idxStr := args[0].String()
		idx := 0
		if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil || idx < 1 {
			return NewText(""), nil
		}
		if val, ok := rt.Scope.Get(fmt.Sprintf("参数%d", idx)); ok {
			return val, nil
		}
		return NewText(""), nil
	})
	// 定义常量 — 全局常量（跨脚本可见）
	rt.RegisterFunc("定义常量", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		rt.Globals.Set(name, args[1])
		rt.Scope.Set(name, args[1])
		return NewNull(), nil
	})

	// 判真 — 判断文本是否为真
	rt.RegisterFunc("判真", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 3 {
			return NewNull(), nil
		}
		if args[0].IsTrue() {
			return args[2], nil
		}
		return args[1], nil
	})

	// 判空 — 如果内容长度为0，返回替换值，否则返回原内容
	rt.RegisterFunc("判空", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		if len(args[0].String()) == 0 {
			return args[1], nil
		}
		return args[0], nil
	})

	// 选择 — 根据数字选择内容（空数字=随机）
	rt.RegisterFunc("选择", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		idxStr := args[0].String()
		if idxStr == "" {
			idx := rand.IntN(len(args) - 1)
			return args[idx+1], nil
		}
		idx := 0
		if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil {
			return NewNull(), nil
		}
		if idx < 0 || idx >= len(args)-1 {
			return NewNull(), nil
		}
		return args[idx+1], nil
	})

	// 逻辑选择 — 根据逻辑数组中第一个真的位置选择
	rt.RegisterFunc("逻辑选择", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		// 第一个参数：逻辑数组（只包含"真"/"假"值）
		boolArr := args[0]
		if boolArr.Type != ValArray {
			return NewNull(), nil
		}
		for i, val := range boolArr.Array {
			if val.IsTrue() {
				idx := i + 1
				if idx < len(args) {
					return args[idx], nil
				}
				break
			}
		}
		return NewNull(), nil
	})

	// 当前版本 — 返回当前版本
	rt.RegisterFunc("当前版本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText("0.0.1"), nil
	})

	// 换行 — 换行符
	rt.RegisterFunc("换行", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText("\n"), nil
	})

	// 回车 — 回车符
	rt.RegisterFunc("回车", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText("\r"), nil
	})

	// 空格 — 空格
	rt.RegisterFunc("空格", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText(" "), nil
	})

	// 随机取 — 从参数中随机取一个
	rt.RegisterFunc("随机取", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) == 0 {
			return NewText(""), nil
		}
		idx := rand.IntN(len(args))
		return args[idx], nil
	})

	// 隐藏 — 隐藏输出
	rt.RegisterFunc("隐藏", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) > 0 {
			// 存储被隐藏的内容
			rt.Scope.Set("__hidden__", args[0])
		}
		return NewNull(), nil
	})

	// 传递 — 取出被隐藏的内容
	rt.RegisterFunc("传递", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__hidden__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 屏蔽 — 屏蔽命令输出（与隐藏相同）
	rt.RegisterFunc("屏蔽", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) > 0 {
			rt.Scope.Set("__hidden__", args[0])
		}
		return NewNull(), nil
	})
	// ---- Phase 2: 数据容器与实用命令 ----

	// 数组 — 构建数组
	rt.RegisterFunc("数组", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewArray(args), nil
	})

	// 对象 — 构建对象（key1, val1, key2, val2, ...）
	rt.RegisterFunc("对象", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		obj := make(map[string]*RedValue)
		for i := 0; i+1 < len(args); i += 2 {
			obj[args[i].String()] = args[i+1]
		}
		return NewObject(obj), nil
	})

	// 取长度 — 多态取长度
	rt.RegisterFunc("取长度", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText("0"), nil
		}
		val := args[0]
		switch val.Type {
		case ValText:
			return NewText(strconv.Itoa(len([]rune(val.Text)))), nil
		case ValArray:
			return NewText(strconv.Itoa(len(val.Array))), nil
		case ValObject:
			return NewText(strconv.Itoa(len(val.Object))), nil
		case ValBin:
			return NewText(strconv.Itoa(len(val.Bin))), nil
		default:
			return NewText("0"), nil
		}
	})

	// 取元素 — 多态取元素（内容, 下标1, 下标2, ...）
	rt.RegisterFunc("取元素", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		current := args[0]
		for i := 1; i < len(args); i++ {
			idx := args[i].String()
			switch current.Type {
			case ValArray:
				n, err := strconv.Atoi(idx)
				if err != nil || n < 0 || n >= len(current.Array) {
					return NewText(""), nil
				}
				current = current.Array[n]
			case ValObject:
				if v, ok := current.Object[idx]; ok {
					current = v
				} else {
					return NewText(""), nil
				}
			case ValText:
				runes := []rune(current.Text)
				n, err := strconv.Atoi(idx)
				if err != nil || n < 0 || n >= len(runes) {
					return NewText(""), nil
				}
				current = NewText(string(runes[n]))
			default:
				return NewText(""), nil
			}
		}
		return current, nil
	})

	// 取变量元素 — 对变量取元素
	rt.RegisterFunc("取变量元素", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		val, ok := rt.Scope.Get(name)
		if !ok {
			return NewText(""), nil
		}
		idx := args[1].String()
		switch val.Type {
		case ValArray:
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(val.Array) {
				return NewText(""), nil
			}
			return val.Array[n], nil
		case ValObject:
			if v, ok := val.Object[idx]; ok {
				return v, nil
			}
			return NewText(""), nil
		case ValText:
			runes := []rune(val.Text)
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(runes) {
				return NewText(""), nil
			}
			return NewText(string(runes[n])), nil
		case ValBin:
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(val.Bin) {
				return NewBin(nil), nil
			}
			return NewBin([]byte{val.Bin[n]}), nil
		default:
			return NewText(""), nil
		}
	})

	// 增加元素 — 向变量添加元素
	rt.RegisterFunc("增加元素", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		val, ok := rt.Scope.Get(name)
		if !ok {
			return NewNull(), nil
		}
		switch val.Type {
		case ValArray:
			val.Array = append(val.Array, args[1:]...)
		case ValObject:
			for i := 1; i+1 < len(args); i += 2 {
				val.Object[args[i].String()] = args[i+1]
			}
		case ValText:
			val.Text += args[1].String()
		case ValBin:
			val.Bin = append(val.Bin, args[1].Bin...)
		}
		return NewNull(), nil
	})

	// 替换元素 — 替换变量中的元素
	rt.RegisterFunc("替换元素", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 3 {
			return NewNull(), nil
		}
		name := args[0].String()
		idx := args[1].String()
		newVal := args[2]
		val, ok := rt.Scope.Get(name)
		if !ok {
			return NewNull(), nil
		}
		switch val.Type {
		case ValArray:
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(val.Array) {
				return NewNull(), nil
			}
			val.Array[n] = newVal
		case ValObject:
			val.Object[idx] = newVal
		case ValText:
			runes := []rune(val.Text)
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(runes) {
				return NewNull(), nil
			}
			newRunes := []rune(newVal.String())
			if len(newRunes) > 0 {
				runes[n] = newRunes[0]
				val.Text = string(runes)
			}
		case ValBin:
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(val.Bin) {
				return NewNull(), nil
			}
			if len(newVal.Bin) > 0 {
				val.Bin[n] = newVal.Bin[0]
			}
		}
		return NewNull(), nil
	})

	// 删除元素 — 删除变量中的元素
	rt.RegisterFunc("删除元素", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		name := args[0].String()
		idx := args[1].String()
		val, ok := rt.Scope.Get(name)
		if !ok {
			return NewNull(), nil
		}
		switch val.Type {
		case ValArray:
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(val.Array) {
				return NewNull(), nil
			}
			val.Array = append(val.Array[:n], val.Array[n+1:]...)
		case ValObject:
			delete(val.Object, idx)
		case ValText:
			runes := []rune(val.Text)
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(runes) {
				return NewNull(), nil
			}
			val.Text = string(append(runes[:n], runes[n+1:]...))
		case ValBin:
			n, err := strconv.Atoi(idx)
			if err != nil || n < 0 || n >= len(val.Bin) {
				return NewNull(), nil
			}
			val.Bin = append(val.Bin[:n], val.Bin[n+1:]...)
		}
		return NewNull(), nil
	})

	// 取对象key — 获取对象的所有key
	rt.RegisterFunc("取对象key", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewArray(nil), nil
		}
		val := args[0]
		if val.Type != ValObject {
			return NewArray(nil), nil
		}
		keys := make([]*RedValue, 0, len(val.Object))
		for k := range val.Object {
			keys = append(keys, NewText(k))
		}
		return NewArray(keys), nil
	})

	// 转文本 — 将值转为文本
	rt.RegisterFunc("转文本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		val := args[0]
		switch val.Type {
		case ValText:
			return val, nil
		case ValArray:
			b, _ := json.Marshal(val.ToSimple())
			return NewText(string(b)), nil
		case ValObject:
			b, _ := json.Marshal(val.ToSimple())
			return NewText(string(b)), nil
		case ValBin:
			encoding := "UTF8"
			if len(args) > 1 {
				encoding = args[1].String()
			}
			if strings.EqualFold(encoding, "GBK") {
				return NewText(string(val.Bin)), nil
			}
			return NewText(string(val.Bin)), nil
		default:
			return NewText(val.String()), nil
		}
	})

	// 入栈 — 将内容入栈
	rt.RegisterFunc("入栈", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		stack := getStack(rt)
		*stack = append(*stack, args[0])
		return NewNull(), nil
	})

	// 出栈 — 出栈
	rt.RegisterFunc("出栈", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		stack := getStack(rt)
		if len(*stack) == 0 {
			return NewText(""), nil
		}
		val := (*stack)[len(*stack)-1]
		*stack = (*stack)[:len(*stack)-1]
		return val, nil
	})

	// 栈顶 — 查看栈顶元素
	rt.RegisterFunc("栈顶", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		stack := getStack(rt)
		idx := len(*stack) - 1
		if len(args) > 0 {
			n, err := strconv.Atoi(args[0].String())
			if err == nil && n >= 0 && n < len(*stack) {
				idx = len(*stack) - 1 - n
			}
		}
		if idx < 0 || idx >= len(*stack) {
			return NewText(""), nil
		}
		return (*stack)[idx], nil
	})

	// 栈长度 — 返回栈的长度
	rt.RegisterFunc("栈长度", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		stack := getStack(rt)
		return NewText(strconv.Itoa(len(*stack))), nil
	})

	// 正则 — 正则匹配
	rt.RegisterFunc("正则", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewText(""), nil
		}
		re, err := regexp.Compile(args[1].String())
		if err != nil {
			return NewText(""), nil
		}
		matches := re.FindStringSubmatch(args[0].String())
		if len(matches) == 0 {
			return NewText(""), nil
		}
		// 保存匹配结果到作用域（供 取子匹配 使用）
		rt.Scope.Set("__regex_matches__", NewArray(strsToVals(matches)))
		return NewText(matches[0]), nil
	})

	// 子匹配数量 — 返回子匹配数量
	rt.RegisterFunc("子匹配数量", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__regex_matches__"); ok && val.Type == ValArray {
			return NewText(strconv.Itoa(len(val.Array) - 1)), nil
		}
		return NewText("0"), nil
	})

	// 取子匹配 — 取第 N 个子匹配
	rt.RegisterFunc("取子匹配", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		n, err := strconv.Atoi(args[0].String())
		if err != nil {
			return NewText(""), nil
		}
		if val, ok := rt.Scope.Get("__regex_matches__"); ok && val.Type == ValArray {
			if n >= 0 && n < len(val.Array) {
				return val.Array[n], nil
			}
		}
		return NewText(""), nil
	})

	// 编码 — URL 编码
	rt.RegisterFunc("编码", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		return NewText(url.QueryEscape(args[0].String())), nil
	})

	// URL 解码
	rt.RegisterFunc("URL解码", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		s, err := url.QueryUnescape(args[0].String())
		if err != nil {
			return NewText(""), nil
		}
		return NewText(s), nil
	})

	// Base64 编码
	rt.RegisterFunc("Base64编码", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		return NewText(base64.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	})

	// Base64 解码
	rt.RegisterFunc("Base64解码", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		b, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			return NewText(""), nil
		}
		return NewText(string(b)), nil
	})

	// 取时间戳 — 返回当前 Unix 时间戳
	rt.RegisterFunc("取时间戳", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText(strconv.FormatInt(time.Now().Unix(), 10)), nil
	})

	// 取时间 — 返回当前时间文本
	rt.RegisterFunc("取时间", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText(time.Now().Format("15:04:05")), nil
	})

	// 取日期 — 返回当前日期文本
	rt.RegisterFunc("取日期", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		return NewText(time.Now().Format("2006-01-02")), nil
	})

	// 格式化时间 — 自定义格式
	rt.RegisterFunc("格式化时间", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(time.Now().Format("2006-01-02 15:04:05")), nil
		}
		return NewText(time.Now().Format(args[0].String())), nil
	})

	// ---- Phase 3: 外部交互命令 ----

	// 读文件 — 读取文件内容
	rt.RegisterFunc("读文件", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		path := args[0].String()
		data, err := os.ReadFile(path)
		if err != nil {
			return NewText(""), nil
		}
		return NewText(string(data)), nil
	})

	// 写文件 — 写入文件内容
	rt.RegisterFunc("写文件", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		path := args[0].String()
		content := args[1].String()
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// 文件是否存在
	rt.RegisterFunc("文件是否存在", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText("假"), nil
		}
		_, err := os.Stat(args[0].String())
		if err == nil {
			return NewText("真"), nil
		}
		return NewText("假"), nil
	})

	// 取后缀 — 取文件后缀
	rt.RegisterFunc("取后缀", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		ext := filepath.Ext(args[0].String())
		return NewText(ext), nil
	})

	// 应用目录 — 返回当前工作目录
	rt.RegisterFunc("应用目录", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		dir, err := os.Getwd()
		if err != nil {
			return NewText(""), nil
		}
		return NewText(dir), nil
	})

	// 取目录 — 取路径的目录部分
	rt.RegisterFunc("取目录", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		dir := filepath.Dir(args[0].String())
		return NewText(dir), nil
	})

	// 创建目录
	rt.RegisterFunc("创建目录", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		if err := os.MkdirAll(args[0].String(), 0755); err != nil {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// 发送 HTTP 请求
	rt.RegisterFunc("发送HTTP请求", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 参数: url, [method, [body]]
		if len(args) < 1 {
			return NewText(""), nil
		}
		urlStr := args[0].String()
		method := "GET"
		if len(args) > 1 && args[1].String() != "" {
			method = strings.ToUpper(args[1].String())
		}
		var body io.Reader
		if len(args) > 2 {
			body = strings.NewReader(args[2].String())
		}
		req, err := http.NewRequest(method, urlStr, body)
		if err != nil {
			return NewText(""), nil
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return NewText(""), nil
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return NewText(""), nil
		}
		return NewText(string(data)), nil
	})

	// 文件下载 — 下载文件到本地
	rt.RegisterFunc("文件下载", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		urlStr := args[0].String()
		savePath := args[1].String()
		resp, err := http.Get(urlStr)
		if err != nil {
			return NewNull(), nil
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return NewNull(), nil
		}
		if err := os.WriteFile(savePath, data, 0644); err != nil {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// json 解析 — 将 JSON 字符串解析为 RedLang 对象
	rt.RegisterFunc("json解析", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		var data any
		if err := json.Unmarshal([]byte(args[0].String()), &data); err != nil {
			return NewNull(), nil
		}
		return jsonToRedValue(data), nil
	})

	// json 序列化 — 将 RedLang 值转为 JSON 字符串
	rt.RegisterFunc("json序列化", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		simple := args[0].ToSimple()
		b, err := json.Marshal(simple)
		if err != nil {
			return NewText(""), nil
		}
		return NewText(string(b)), nil
	})

	// ---- Phase 4: 机器人交互与事件系统命令 ----

	// 机器人ID — 当前机器人的ID
	rt.RegisterFunc("机器人ID", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__self_id__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 机器人名字 — 当前机器人的名字
	rt.RegisterFunc("机器人名字", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__bot_name__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 机器人平台 — 当前机器人的平台
	rt.RegisterFunc("机器人平台", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__platform__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 当前消息 — 当前的消息内容
	rt.RegisterFunc("当前消息", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__message__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 发送者ID — 发送者的用户ID
	rt.RegisterFunc("发送者ID", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__user_id__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 发送者昵称 — 发送者的昵称
	rt.RegisterFunc("发送者昵称", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__user_name__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 群ID — 当前群组的ID
	rt.RegisterFunc("群ID", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__group_id__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 群名称 — 当前群组的名称
	rt.RegisterFunc("群名称", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__group_name__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 发送者权限 — 发送者在当前群中的权限
	rt.RegisterFunc("发送者权限", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__user_role__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 事件内容 — onebot事件json对应的RedLang对象
	rt.RegisterFunc("事件内容", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__event_raw__"); ok {
			return val, nil
		}
		return NewNull(), nil
	})

	// 消息ID — 当前消息的ID
	rt.RegisterFunc("消息ID", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__message_id__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 设置来源 — 设置回复的目标
	rt.RegisterFunc("设置来源", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		rt.Scope.Set("__reply_platform__", args[0])
		rt.Scope.Set("__reply_target__", args[1])
		return NewNull(), nil
	})

	// 发送 — 发送消息（自动使用设置来源的信息）
	rt.RegisterFunc("发送", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		userID, _ := rt.Scope.Get("__user_id__")
		groupID, _ := rt.Scope.Get("__group_id__")
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		// 检查是否有设置来源
		if replyTarget, ok := rt.Scope.Get("__reply_target__"); ok && replyTarget.String() != "" {
			groupID = replyTarget
		}
		if replyPlatform, ok := rt.Scope.Get("__reply_platform__"); ok && replyPlatform.String() != "" {
			platform = replyPlatform
		}

		params := make(map[string]any)
		params["message"] = args[0].String()
		if groupID.String() != "" {
			params["group_id"] = groupID.String()
			params["message_type"] = "group"
		} else if userID.String() != "" {
			params["user_id"] = userID.String()
			params["message_type"] = "private"
		}

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			GlobalApp.BotAPI(platform.String(), selfID.String(), "send_msg", params)
		}
		return NewNull(), nil
	})

	// 取图片 — 取消息中的图片URL数组
	rt.RegisterFunc("取图片", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		text := ""
		if len(args) > 0 {
			text = args[0].String()
		} else if val, ok := rt.Scope.Get("__message__"); ok {
			text = val.String()
		}
		// 简单CQ码图片提取
		var urls []*RedValue
		for {
			start := strings.Index(text, "[CQ:image,file=")
			if start < 0 {
				break
			}
			text = text[start+len("[CQ:image,file="):]
			end := strings.IndexAny(text, ",]")
			if end < 0 {
				break
			}
			urls = append(urls, NewText(text[:end]))
			text = text[end:]
		}
		return NewArray(urls), nil
	})

	// 取艾特 — 取消息中被艾特的人
	rt.RegisterFunc("取艾特", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		text := ""
		if len(args) > 0 {
			text = args[0].String()
		} else if val, ok := rt.Scope.Get("__message__"); ok {
			text = val.String()
		}
		var ids []*RedValue
		for {
			start := strings.Index(text, "[CQ:at,qq=")
			if start < 0 {
				break
			}
			text = text[start+len("[CQ:at,qq="):]
			end := strings.IndexAny(text, ",]")
			if end < 0 {
				break
			}
			ids = append(ids, NewText(text[:end]))
			text = text[end:]
		}
		return NewArray(ids), nil
	})

	// 取文本 — 取消息中的纯文本数组
	rt.RegisterFunc("取文本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		text := ""
		if len(args) > 0 {
			text = args[0].String()
		} else if val, ok := rt.Scope.Get("__message__"); ok {
			text = val.String()
		}
		// 移除CQ码，返回纯文本
		var texts []*RedValue
		current := text
		for {
			start := strings.Index(current, "[CQ:")
			if start < 0 {
				if current != "" {
					texts = append(texts, NewText(strings.TrimSpace(current)))
				}
				break
			}
			if start > 0 {
				texts = append(texts, NewText(strings.TrimSpace(current[:start])))
			}
			end := strings.Index(current[start:], "]")
			if end < 0 {
				break
			}
			current = current[start+end+1:]
		}
		return NewArray(texts), nil
	})

	// OB调用 — 发送原始OneBot数据
	rt.RegisterFunc("OB调用", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		jsonStr := args[0].String()
		var params map[string]any
		if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
			return NewNull(), nil
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		action := ""
		if a, ok := params["action"]; ok {
			action = fmt.Sprintf("%v", a)
			delete(params, "action")
		}

		if GlobalApp != nil && GlobalApp.BotAPI != nil && action != "" {
			result, err := GlobalApp.BotAPI(platform.String(), selfID.String(), action, params)
			if err != nil {
				return NewText(""), nil
			}
			return jsonToRedValue(result), nil
		}
		return NewNull(), nil
	})

	// 同意 — 同意好友/群请求
	rt.RegisterFunc("同意", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		remark := ""
		if len(args) > 0 {
			remark = args[0].String()
		}
		flag, _ := rt.Scope.Get("__flag__")
		eventType, _ := rt.Scope.Get("__event_type__")
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		params := map[string]any{"flag": flag.String()}
		if remark != "" {
			params["remark"] = remark
		}

		action := ""
		switch eventType.String() {
		case "request:group:add", "request:group:invite":
			action = "set_group_add_request"
		case "request:friend":
			action = "set_friend_add_request"
		}

		if GlobalApp != nil && GlobalApp.BotAPI != nil && action != "" {
			params["approve"] = true
			GlobalApp.BotAPI(platform.String(), selfID.String(), action, params)
		}
		return NewNull(), nil
	})

	// 拒绝 — 拒绝好友/群请求
	rt.RegisterFunc("拒绝", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		reason := ""
		if len(args) > 0 {
			reason = args[0].String()
		}
		flag, _ := rt.Scope.Get("__flag__")
		eventType, _ := rt.Scope.Get("__event_type__")
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		params := map[string]any{"flag": flag.String()}
		if reason != "" {
			params["reason"] = reason
		}

		action := ""
		switch eventType.String() {
		case "request:group:add", "request:group:invite":
			action = "set_group_add_request"
		case "request:friend":
			action = "set_friend_add_request"
		}

		if GlobalApp != nil && GlobalApp.BotAPI != nil && action != "" {
			params["approve"] = false
			GlobalApp.BotAPI(platform.String(), selfID.String(), action, params)
		}
		return NewNull(), nil
	})

	// 表情回应 — 对消息进行表情回应
	rt.RegisterFunc("表情回应", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		emoji := args[0].String()
		msgID, _ := rt.Scope.Get("__message_id__")
		if len(args) > 1 {
			msgID = args[1]
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			GlobalApp.BotAPI(platform.String(), selfID.String(), "set_message_reaction", map[string]any{
				"message_id": msgID.String(),
				"emoji":      emoji,
			})
		}
		return NewNull(), nil
	})

	// 输入流 — 获取当前群/发送者的下一条消息
	rt.RegisterFunc("输入流", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 输入流暂不支持同步实现，返回空
		return NewText(""), nil
	})

	// 设置主人 — 设置机器人主人
	rt.RegisterFunc("设置主人", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		// 将主人列表存到作用域
		rt.Scope.Set("__masters__", args[0])
		// 也存到全局
		if GlobalApp != nil && GlobalApp.SetVar != nil {
			GlobalApp.SetVar("__masters__", args[0].String())
		}
		return NewNull(), nil
	})

	// 主人数组 — 返回主人ID数组
	rt.RegisterFunc("主人数组", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__masters__"); ok {
			return val, nil
		}
		if GlobalApp != nil && GlobalApp.GetVar != nil {
			masterStr := GlobalApp.GetVar("__masters__")
			if masterStr != "" {
				return NewText(masterStr), nil
			}
		}
		return NewArray(nil), nil
	})

	// 主人 — 检查发送者是否为主人
	rt.RegisterFunc("主人", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		userID := ""
		if len(args) > 0 {
			userID = args[0].String()
		} else if val, ok := rt.Scope.Get("__user_id__"); ok {
			userID = val.String()
		}
		// 如果是主人，返回空，否则返回（用于退出）
		if isMaster(userID, rt) {
			return NewNull(), nil
		}
		rt.retSignal = ReturnNow
		return NewNull(), nil
	})

	// 取群员 — 取群成员信息
	rt.RegisterFunc("取群员", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		groupID, _ := rt.Scope.Get("__group_id__")
		userID, _ := rt.Scope.Get("__user_id__")
		if len(args) > 0 {
			userID = args[0]
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			result, err := GlobalApp.BotAPI(platform.String(), selfID.String(), "get_group_member_info", map[string]any{
				"group_id": groupID.String(),
				"user_id":  userID.String(),
			})
			if err != nil {
				return NewNull(), nil
			}
			return jsonToRedValue(result), nil
		}
		return NewNull(), nil
	})

	// 取群员列表 — 取群成员列表
	rt.RegisterFunc("取群员列表", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		groupID, _ := rt.Scope.Get("__group_id__")
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			result, err := GlobalApp.BotAPI(platform.String(), selfID.String(), "get_group_member_list", map[string]any{
				"group_id": groupID.String(),
			})
			if err != nil {
				return NewNull(), nil
			}
			return jsonToRedValue(result), nil
		}
		return NewNull(), nil
	})

	// 取群列表 — 取群列表
	rt.RegisterFunc("取群列表", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			result, err := GlobalApp.BotAPI(platform.String(), selfID.String(), "get_group_list", nil)
			if err != nil {
				return NewNull(), nil
			}
			return jsonToRedValue(result), nil
		}
		return NewNull(), nil
	})

	// 禁止发言 — 禁言群成员
	rt.RegisterFunc("禁止发言", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		groupID, _ := rt.Scope.Get("__group_id__")
		userID, _ := rt.Scope.Get("__user_id__")
		duration := int64(600)
		if len(args) > 0 {
			userID = args[0]
		}
		if len(args) > 1 {
			n, err := strconv.ParseInt(args[1].String(), 10, 64)
			if err == nil {
				duration = n
			}
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			GlobalApp.BotAPI(platform.String(), selfID.String(), "set_group_ban", map[string]any{
				"group_id":  groupID.String(),
				"user_id":   userID.String(),
				"duration":  duration,
			})
		}
		return NewNull(), nil
	})

	// 解除禁止发言 — 解除禁言
	rt.RegisterFunc("解除禁止发言", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		groupID, _ := rt.Scope.Get("__group_id__")
		userID, _ := rt.Scope.Get("__user_id__")
		if len(args) > 0 {
			userID = args[0]
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			GlobalApp.BotAPI(platform.String(), selfID.String(), "set_group_ban", map[string]any{
				"group_id": groupID.String(),
				"user_id":  userID.String(),
				"duration": 0,
			})
		}
		return NewNull(), nil
	})

	// 踢出群 — 踢出群成员
	rt.RegisterFunc("踢出群", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		groupID, _ := rt.Scope.Get("__group_id__")
		userID, _ := rt.Scope.Get("__user_id__")
		rejectAdd := true
		if len(args) > 0 {
			userID = args[0]
		}
		if len(args) > 1 && args[1].String() == "假" {
			rejectAdd = false
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			GlobalApp.BotAPI(platform.String(), selfID.String(), "set_group_kick", map[string]any{
				"group_id":          groupID.String(),
				"user_id":           userID.String(),
				"reject_add_request": rejectAdd,
			})
		}
		return NewNull(), nil
	})

	// 设置群名片 — 设置群成员名片
	rt.RegisterFunc("设置群名片", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		groupID, _ := rt.Scope.Get("__group_id__")
		userID, _ := rt.Scope.Get("__user_id__")
		card := ""
		if len(args) > 0 {
			card = args[0].String()
		}
		if len(args) > 1 {
			userID = args[1]
		}
		platform, _ := rt.Scope.Get("__platform__")
		selfID, _ := rt.Scope.Get("__self_id__")

		if GlobalApp != nil && GlobalApp.BotAPI != nil {
			GlobalApp.BotAPI(platform.String(), selfID.String(), "set_group_card", map[string]any{
				"group_id": groupID.String(),
				"user_id":  userID.String(),
				"card":     card,
			})
		}
		return NewNull(), nil
	})

	// 取回复ID — 取消息中的回复ID
	rt.RegisterFunc("取回复ID", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		text := ""
		if len(args) > 0 {
			text = args[0].String()
		} else if val, ok := rt.Scope.Get("__message__"); ok {
			text = val.String()
		}
		start := strings.Index(text, "[CQ:reply,id=")
		if start < 0 {
			return NewText(""), nil
		}
		text = text[start+len("[CQ:reply,id="):]
		end := strings.IndexAny(text, ",]")
		if end < 0 {
			return NewText(""), nil
		}
		return NewText(text[:end]), nil
	})

	// ---- Phase 5: 高级特性命令 ----

	// 错误信息 — 获取脚本错误信息
	rt.RegisterFunc("错误信息", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__error__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 重定向 — 跳转到另一个脚本处理
	rt.RegisterFunc("重定向", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		scriptName := args[0].String()
		// 交由调度器处理，目前为桩
		if GlobalApp != nil && GlobalApp.Scripts != nil {
			scripts := GlobalApp.Scripts()
			for _, s := range scripts {
				if s.ScriptName == scriptName {
					// 重新解析执行目标脚本
					innerAst, err := Parse(s.Code)
					if err == nil {
						return rt.Eval(innerAst)
					}
				}
			}
		}
		return NewNull(), nil
	})

	// 读词库文件 — 读取铃心兼容的词库
	rt.RegisterFunc("读词库文件", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		path := args[0].String()
		data, err := os.ReadFile(path)
		if err != nil {
			return NewNull(), nil
		}
		result := make(map[string]*RedValue)
		lines := strings.Split(string(data), "\n")
		var currentKey string
		var currentValues []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				if currentKey != "" {
					vals := make([]*RedValue, len(currentValues))
					for i, v := range currentValues {
						vals[i] = NewText(v)
					}
					result[currentKey] = NewArray(vals)
					currentKey = ""
					currentValues = nil
				}
				continue
			}
			if currentKey == "" {
				currentKey = line
			} else {
				currentValues = append(currentValues, line)
			}
		}
		// 处理最后一组
		if currentKey != "" {
			vals := make([]*RedValue, len(currentValues))
			for i, v := range currentValues {
				vals[i] = NewText(v)
			}
			result[currentKey] = NewArray(vals)
		}
		return NewObject(result), nil
	})

	// 积分 — 获取发送者在当前群的积分
	rt.RegisterFunc("积分", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		userID := ""
		groupID := ""
		if v, ok := rt.Scope.Get("__user_id__"); ok && v != nil {
			userID = v.String()
		}
		if v, ok := rt.Scope.Get("__group_id__"); ok && v != nil {
			groupID = v.String()
		}
		key := fmt.Sprintf("__score_%s_%s", userID, groupID)
		if val, ok := rt.Scope.Get(key); ok {
			return val, nil
		}
		if GlobalApp != nil && GlobalApp.GetVar != nil {
			s := GlobalApp.GetVar(key)
			if s != "" {
				return NewText(s), nil
			}
		}
		return NewText("0"), nil
	})

	// 积分-增加 — 增加积分
	rt.RegisterFunc("积分-增加", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		userID := ""
		groupID := ""
		if v, ok := rt.Scope.Get("__user_id__"); ok && v != nil {
			userID = v.String()
		}
		if v, ok := rt.Scope.Get("__group_id__"); ok && v != nil {
			groupID = v.String()
		}
		key := fmt.Sprintf("__score_%s_%s", userID, groupID)
		current := int64(0)
		if val, ok := rt.Scope.Get(key); ok {
			current, _ = strconv.ParseInt(val.String(), 10, 64)
		} else if GlobalApp != nil && GlobalApp.GetVar != nil {
			s := GlobalApp.GetVar(key)
			if s != "" {
				current, _ = strconv.ParseInt(s, 10, 64)
			}
		}
		delta, _ := strconv.ParseInt(args[0].String(), 10, 64)
		newScore := current + delta
		if newScore < 0 {
			newScore = 0
		}
		rt.Scope.Set(key, NewText(strconv.FormatInt(newScore, 10)))
		if GlobalApp != nil && GlobalApp.SetVar != nil {
			GlobalApp.SetVar(key, strconv.FormatInt(newScore, 10))
		}
		return NewNull(), nil
	})

	// 积分-设置 — 设置积分
	rt.RegisterFunc("积分-设置", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		userID := ""
		groupID := ""
		if v, ok := rt.Scope.Get("__user_id__"); ok && v != nil {
			userID = v.String()
		}
		if v, ok := rt.Scope.Get("__group_id__"); ok && v != nil {
			groupID = v.String()
		}
		key := fmt.Sprintf("__score_%s_%s", userID, groupID)
		n, _ := strconv.ParseInt(args[0].String(), 10, 64)
		if n < 0 {
			n = 0
		}
		rt.Scope.Set(key, NewText(strconv.FormatInt(n, 10)))
		if GlobalApp != nil && GlobalApp.SetVar != nil {
			GlobalApp.SetVar(key, strconv.FormatInt(n, 10))
		}
		return NewNull(), nil
	})

	// 积分-排行 — 获取当前群积分排行
	rt.RegisterFunc("积分-排行", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 暂不实现（需要扫描所有用户积分），返回空数组
		return NewArray(nil), nil
	})

	// github代理 — 获取可用的github代理
	rt.RegisterFunc("github代理", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 返回空（表示当前环境不需要代理）
		return NewText(""), nil
	})

	// 设置桌面背景 — 仅Windows可用
	rt.RegisterFunc("设置桌面背景", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		// 暂不实现跨平台版本
		return NewNull(), nil
	})

	// ---- 网络触发相关命令 ----

	// 网络-访问参数
	rt.RegisterFunc("网络-访问参数", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__net_params__"); ok {
			return val, nil
		}
		return NewNull(), nil
	})

	// 网络-访问体
	rt.RegisterFunc("网络-访问体", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__net_body__"); ok {
			return val, nil
		}
		return NewBin(nil), nil
	})

	// 网络-访问头
	rt.RegisterFunc("网络-访问头", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__net_headers__"); ok {
			return val, nil
		}
		return NewNull(), nil
	})

	// 网络-设置返回头
	rt.RegisterFunc("网络-设置返回头", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		// 存储响应头设置
		key := args[0].String()
		val := args[1].String()
		headers, ok := rt.Scope.Get("__net_resp_headers__")
		if !ok {
			headers = NewObject(make(map[string]*RedValue))
			rt.Scope.Set("__net_resp_headers__", headers)
		}
		headers.Object[key] = NewText(val)
		return NewNull(), nil
	})

	// 网络-访问方法
	rt.RegisterFunc("网络-访问方法", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__net_method__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 网络-权限
	rt.RegisterFunc("网络-权限", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__net_auth__"); ok {
			return val, nil
		}
		return NewText("只读"), nil
	})

	// ---- AI相关命令 ----

	// GPT-创建单轮对话
	rt.RegisterFunc("GPT-创建单轮对话", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 3 {
			return NewText(""), nil
		}
		endpoint := args[0].String()
		apiKey := args[1].String()
		model := args[2].String()
		// 创建GPT会话结构体（指针为字符串）
		gptID := fmt.Sprintf("gpt_%d", time.Now().UnixNano())
		gptCtx := map[string]any{
			"endpoint": endpoint,
			"api_key":  apiKey,
			"model":    model,
			"messages": []map[string]string{},
		}
		// 存到作用域
		rt.Scope.Set(gptID, jsonToRedValue(gptCtx))
		return NewText(gptID), nil
	})

	// GPT-增加文本
	rt.RegisterFunc("GPT-增加文本", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// GPT-增加图片
	rt.RegisterFunc("GPT-增加图片", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 2 {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// GPT-发送请求
	rt.RegisterFunc("GPT-发送请求", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// GPT-获取回复
	rt.RegisterFunc("GPT-获取回复", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewText(""), nil
		}
		return NewText(""), nil
	})

	// GPT-删除指针
	rt.RegisterFunc("GPT-删除指针", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		return NewNull(), nil
	})

	// 邮件主题 — 仅在邮件平台有效
	rt.RegisterFunc("邮件主题", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if val, ok := rt.Scope.Get("__email_subject__"); ok {
			return val, nil
		}
		return NewText(""), nil
	})

	// 脚本输出 — 获取脚本发送的消息ID数组
	rt.RegisterFunc("脚本输出", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewArray(nil), nil
		}
		msgID := args[0].String()
		// 查找关联的消息ID
		key := fmt.Sprintf("__output_%s__", msgID)
		if val, ok := rt.Scope.Get(key); ok {
			return val, nil
		}
		return NewArray(nil), nil
	})

	// 脚本输出-增加ID
	rt.RegisterFunc("脚本输出-增加ID", func(args []*RedValue, rt *Runtime) (*RedValue, error) {
		if len(args) < 1 {
			return NewNull(), nil
		}
		msgID := args[0].String()
		key := fmt.Sprintf("__output_%s__", msgID)
		val, ok := rt.Scope.Get(key)
		if !ok {
			val = NewArray([]*RedValue{})
		}
		if val.Type == ValArray {
			val.Array = append(val.Array, NewText(msgID))
		}
		rt.Scope.Set(key, val)
		return NewNull(), nil
	})
}


