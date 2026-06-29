package redlang

import (
	"fmt"
	"strconv"
	"strings"

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

// Set 设置变量
func (s *Scope) Set(name string, v *RedValue) {
	s.vars[name] = v
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

// Runtime 是 RedLang 脚本运行时
type Runtime struct {
	Scope       *Scope
	Globals     *Scope
	Functions   map[string]*BuiltinFunc
	loopControl LoopSignal
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
	// 先求值所有参数
	var evaluatedArgs []*RedValue
	for _, argAst := range cmd.Args {
		val, err := rt.Eval(argAst)
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
	// 同时支持按名称绑定（如果函数定义时指定了参数名，后续可扩展）
	oldScope := rt.Scope
	rt.Scope = childScope
	result, err := rt.Eval(*val.Fun)
	rt.Scope = oldScope
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
}
