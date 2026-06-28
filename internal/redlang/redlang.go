package redlang

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// AppContext 是 RedLang 脚本可访问的应用上下文
type AppContext struct {
	mu sync.RWMutex

	// BotAPI 提供机器人 API 调用能力
	BotAPI func(platform, selfID, action string, params map[string]any) (map[string]any, error)

	// LogFunc 日志回调
	LogFunc func(level string, msg string)

	// GetVar / SetVar 全局变量访问
	GetVar func(key string) string
	SetVar func(key, value string)

	// Scripts 当前加载的脚本列表
	Scripts func() []ScriptInfo
}

// ScriptInfo 脚本信息
type ScriptInfo struct {
	PkgName    string
	ScriptName string
	Code       string
	Enabled    bool
}

// GlobalApp 是全局应用上下文
var GlobalApp *AppContext

func init() {
	GlobalApp = &AppContext{
		BotAPI: func(platform, selfID, action string, params map[string]any) (map[string]any, error) {
			return nil, errors.New("BotAPI not initialized")
		},
		LogFunc: func(level, msg string) {
			fmt.Printf("[%s] %s\n", level, msg)
		},
		GetVar: func(key string) string { return "" },
		SetVar: func(key, value string) {},
		Scripts: func() []ScriptInfo { return nil },
	}
}

// EvalScript 便捷函数：对脚本字符串求值
func EvalScript(code string) (string, error) {
	rt := NewRuntime()
	val, err := rt.EvalScript(code)
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// EvalScriptWithCtx 带上下文的脚本求值
func EvalScriptWithCtx(code string, ctx map[string]string) (string, error) {
	rt := NewRuntime()
	// 注入上下文变量
	for k, v := range ctx {
		rt.Scope.Set(k, NewText(v))
	}
	val, err := rt.EvalScript(code)
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// CodeToAst 将脚本代码解析为 AST JSON 表示
func CodeToAst(code string) (string, error) {
	ast, err := Parse(code)
	if err != nil {
		return "", fmt.Errorf("解析失败: %w", err)
	}
	return ast.String(), nil
}

// AstToCode 将 AST 反序列化为脚本代码
func AstToCode(astSrc string) (string, error) {
	// 如果输入已经是 RedLang 格式，尝试解析后再序列化
	ast, err := Parse(astSrc)
	if err != nil {
		return "", err
	}
	return ast.String(), nil
}

// ExtractCommandNames 从脚本中提取所有使用的命令名
func ExtractCommandNames(code string) ([]string, error) {
	ast, err := Parse(code)
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool)
	collectCommands(ast, names)
	var result []string
	for name := range names {
		result = append(result, name)
	}
	return result, nil
}

func collectCommands(ast Ast, names map[string]bool) {
	for _, node := range ast {
		if node.Type == TypeCommand {
			names[node.Cmd.Name] = true
			for _, arg := range node.Cmd.Args {
				collectCommands(arg, names)
			}
		}
	}
}

// ValidateCode 验证脚本代码语法
func ValidateCode(code string) error {
	_, err := Parse(code)
	return err
}

// SanitizeCode 清理脚本代码（白名单模式：只允许已注册的内置函数）
func SanitizeCode(code string) string {
	whitelist := builtinCommands()
	// 找到所有 【命令名】 模式
	var result strings.Builder
	i := 0
	runes := []rune(code)
	for i < len(runes) {
		if runes[i] == '【' {
			// 找到匹配的 】 或 @
			j := i + 1
			var name strings.Builder
			for j < len(runes) && runes[j] != '】' && runes[j] != '@' {
				name.WriteRune(runes[j])
				j++
			}
			cmdName := name.String()
			if !whitelist[cmdName] {
				// 不在白名单，替换为已禁用版本
				result.WriteString("【已禁用_" + cmdName + "】")
				// 跳到命令结束（跳过参数）
				depth := 1
				for j < len(runes) && depth > 0 {
					if runes[j] == '【' {
						depth++
					} else if runes[j] == '】' {
						depth--
					}
					j++
					if depth == 0 {
						break
					}
				}
			} else {
				// 在白名单，原样保留
				result.WriteString(string(runes[i:j]))
				// 保留整个命令（包括参数）
				depth := 1
				for j < len(runes) && depth > 0 {
					if runes[j] == '【' {
						depth++
					} else if runes[j] == '】' {
						depth--
					}
					j++
				}
			}
			i = j
		} else {
			result.WriteRune(runes[i])
			i++
		}
	}
	return result.String()
}

// builtinCommands 返回所有内置命令名的白名单
func builtinCommands() map[string]bool {
	return map[string]bool{
		"输出":     true,
		"如果":     true,
		"则":      true,
		"否则":    true,
		"且":      true,
		"或":      true,
		"==":     true,
		"!=":     true,
		">":      true,
		"<":      true,
		">=":     true,
		"<=":     true,
		"加":      true,
		"减":      true,
		"乘":      true,
		"除":      true,
		"模":      true,
		"取文本长度":  true,
		"取数组长度":  true,
		"取数组成员":  true,
		"寻找文本":   true,
		"到文本":    true,
		"合并文本":   true,
		"子文本替换":  true,
		"令":      true,
		"赋值":     true,
		"取":      true,
		"读取":     true,
		"计次循环":   true,
		"计次循环尾":  true,
		"循环":     true,
		"循环尾":    true,
		"跳出":     true,
		"继续":     true,
		"写日志":    true,
	}
}
