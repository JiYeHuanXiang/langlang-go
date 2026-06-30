// Package redlang 提供 RedLang 中文脚本语言的 AST 定义、解析器与运行时。
//
// RedLang 是一种中文编程语言，专为聊天机器人脚本设计。
// 语法示例：
//
//	【如果】@用户名 == "admin" 【则】发送群消息("你好")
//
// 顶层结构由表达式组成，每个表达式可以是：
//   - 纯文本（Text）
//   - 命令调用（Command）
package redlang

import (
	"fmt"
	"strings"
)

// AstNode 是 AST 节点
type AstNode struct {
	Type AstNodeType
	Text string   // 仅 TypeText 时有效
	Cmd  *AstCmd  // 仅 TypeCommand 时有效
}

type AstNodeType int

const (
	TypeText    AstNodeType = iota // 纯文本
	TypeCommand                    // 命令调用
)

// AstCmd 表示一个命令调用： 【命令名】@参数1@参数2
type AstCmd struct {
	Name    string   // 命令名
	Args    []Ast    // 每个参数又是一个 Ast（支持嵌套）
	RawBody string   // 原始函数体文本（仅函数定义时使用）
}

// Ast 是一组表达式的列表（可嵌套）
type Ast []AstNode

// String 将 AST 序列化为 RedLang 源码
func (a Ast) String() string {
	var b strings.Builder
	for _, node := range a {
		switch node.Type {
		case TypeText:
			for _, ch := range node.Text {
				if ch == '\\' || ch == '@' || ch == '【' || ch == '】' || ch == ' ' || ch == '\t' {
					b.WriteRune('\\')
				}
				b.WriteRune(ch)
			}
		case TypeCommand:
			// 格式：【命令名】@参数1@参数2
			b.WriteString("【")
			for _, ch := range node.Cmd.Name {
				if ch == '\\' || ch == '@' || ch == '【' || ch == '】' {
					b.WriteRune('\\')
				}
				b.WriteRune(ch)
			}
			b.WriteString("】")
			if node.Cmd.RawBody != "" {
				b.WriteRune('@')
				b.WriteString(node.Cmd.RawBody)
			} else {
				for _, arg := range node.Cmd.Args {
					b.WriteRune('@')
					b.WriteString(arg.String())
				}
			}
		}
	}
	return b.String()
}

// RedValue 是 RedLang 运行时的值类型
type RedValue struct {
	Type   RedValueType
	Text   string
	Array  []*RedValue
	Object map[string]*RedValue
	Bin    []byte
	Fun    *Ast // 函数体（已解析的 AST）
}

type RedValueType int

const (
	ValText   RedValueType = iota // 字符串
	ValArray                      // 数组
	ValObject                     // 有序对象
	ValBin                        // 二进制
	ValFun                        // 函数
	ValNull                       // 空
	ValBool                       // 布尔（运行时内部使用）
)

// NewText 创建文本值
func NewText(s string) *RedValue {
	return &RedValue{Type: ValText, Text: s}
}

// NewArray 创建数组值
func NewArray(items []*RedValue) *RedValue {
	if items == nil {
		items = []*RedValue{}
	}
	return &RedValue{Type: ValArray, Array: items}
}

// NewObject 创建对象值
func NewObject(kv map[string]*RedValue) *RedValue {
	if kv == nil {
		kv = map[string]*RedValue{}
	}
	return &RedValue{Type: ValObject, Object: kv}
}

// NewBin 创建二进制值
func NewBin(b []byte) *RedValue {
	return &RedValue{Type: ValBin, Bin: b}
}

// NewFun 创建函数值
func NewFun(ast *Ast) *RedValue {
	return &RedValue{Type: ValFun, Fun: ast}
}

// NewNull 创建空值
func NewNull() *RedValue {
	return &RedValue{Type: ValNull}
}

// NewBool 创建布尔值（内部使用）
func NewBool(b bool) *RedValue {
	if b {
		return NewText("真")
	}
	return NewText("假")
}

// IsTrue 判断值是否为"真"
func (v *RedValue) IsTrue() bool {
	if v == nil || v.Type == ValNull {
		return false
	}
	if v.Type == ValText && v.Text == "真" {
		return true
	}
	return false
}

// ToSimple 将 RedValue 转换为简单的 Go 类型（用于 JSON 序列化）
func (v *RedValue) ToSimple() any {
	if v == nil {
		return nil
	}
	switch v.Type {
	case ValText:
		return v.Text
	case ValArray:
		result := make([]any, len(v.Array))
		for i, item := range v.Array {
			result[i] = item.ToSimple()
		}
		return result
	case ValObject:
		result := make(map[string]any)
		for k, val := range v.Object {
			result[k] = val.ToSimple()
		}
		return result
	case ValBin:
		return v.Bin
	case ValFun:
		return "[function]"
	default:
		return v.String()
	}
}

// String 返回值的字符串表示
func (v *RedValue) String() string {
	if v == nil {
		return ""
	}
	switch v.Type {
	case ValText:
		return v.Text
	case ValArray:
		parts := make([]string, len(v.Array))
		for i, item := range v.Array {
			parts[i] = item.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ValObject:
		return "[object]"
	case ValBin:
		return fmt.Sprintf("[bin %d bytes]", len(v.Bin))
	case ValFun:
		return "[function]"
	case ValNull:
		return "空"
	default:
		return ""
	}
}

// Clone 深拷贝一个值
func (v *RedValue) Clone() *RedValue {
	if v == nil {
		return nil
	}
	nv := &RedValue{Type: v.Type}
	switch v.Type {
	case ValText:
		nv.Text = v.Text
	case ValArray:
		nv.Array = make([]*RedValue, len(v.Array))
		for i, item := range v.Array {
			nv.Array[i] = item.Clone()
		}
	case ValObject:
		nv.Object = make(map[string]*RedValue, len(v.Object))
		for k, val := range v.Object {
			nv.Object[k] = val.Clone()
		}
	case ValBin:
		nv.Bin = make([]byte, len(v.Bin))
		copy(nv.Bin, v.Bin)
	case ValFun:
		if v.Fun != nil {
			cp := make(Ast, len(*v.Fun))
			copy(cp, *v.Fun)
			nv.Fun = &cp
		}
	}
	return nv
}
