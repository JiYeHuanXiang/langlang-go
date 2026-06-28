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

// SanitizeCode 清理脚本代码（移除危险模式等）
func SanitizeCode(code string) string {
	// 简单的安全检查：限制某些命令
	// TODO: 实现更完善的沙箱
	disallowed := []string{"执行", "运行", "调用"}
	for _, d := range disallowed {
		code = strings.ReplaceAll(code, "【"+d+"】", "【已禁用_"+d+"】")
	}
	return code
}
