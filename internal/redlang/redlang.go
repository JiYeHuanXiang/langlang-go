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
		"计算":     true,
		"函数定义":   true,
		"调用函数":   true,
		"参数":     true,
		"参数个数":   true,
		"返回":     true,
		"定义变量":   true,
		"赋值变量":   true,
		"变量":     true,
		"定义命令":   true,
		"定义二类命令": true,
		"二类参数":   true,
		"定义常量":   true,
		"判真":     true,
		"判空":     true,
		"选择":     true,
		"逻辑选择":   true,
		"当前版本":   true,
		"换行":     true,
		"回车":     true,
		"空格":     true,
		"随机取":    true,
		"隐藏":     true,
		"传递":     true,
		"屏蔽":     true,
		// Phase 2
		"数组":     true,
		"对象":     true,
		"取长度":    true,
		"取元素":    true,
		"取变量元素":  true,
		"增加元素":   true,
		"替换元素":   true,
		"删除元素":   true,
		"取对象key": true,
		"转文本":    true,
		"入栈":     true,
		"出栈":     true,
		"栈顶":     true,
		"栈长度":    true,
		"正则":     true,
		"子匹配数量":  true,
		"取子匹配":   true,
		"编码":     true,
		"URL解码":  true,
		"Base64编码": true,
		"Base64解码": true,
		"取时间戳":   true,
		"取时间":    true,
		"取日期":    true,
		"格式化时间":  true,
		// Phase 3
		"读文件":    true,
		"写文件":    true,
		"文件是否存在": true,
		"取后缀":    true,
		"应用目录":   true,
		"取目录":    true,
		"创建目录":   true,
		"发送HTTP请求": true,
		"文件下载":   true,
		"json解析":  true,
		"json序列化": true,
		// Phase 4
		"机器人ID":   true,
		"机器人名字":  true,
		"机器人平台":  true,
		"当前消息":   true,
		"发送者ID":  true,
		"发送者昵称":  true,
		"群ID":    true,
		"群名称":   true,
		"发送者权限":  true,
		"事件内容":   true,
		"消息ID":   true,
		"设置来源":   true,
		"发送":    true,
		"取图片":    true,
		"取艾特":    true,
		"取文本":    true,
		"OB调用":   true,
		"同意":     true,
		"拒绝":     true,
		"表情回应":   true,
		"输入流":    true,
		"设置主人":   true,
		"主人数组":   true,
		"主人":     true,
		"取群员":    true,
		"取群员列表":  true,
		"取群列表":   true,
		"禁止发言":   true,
		"解除禁止发言": true,
		"踢出群":    true,
		"设置群名片":  true,
		"取回复ID":  true,
		// Phase 5
		"错误信息":    true,
		"重定向":    true,
		"读词库文件":  true,
		"积分":     true,
		"积分-增加":  true,
		"积分-设置":  true,
		"积分-排行":  true,
		"github代理": true,
		"设置桌面背景": true,
		"网络-访问参数": true,
		"网络-访问体":  true,
		"网络-访问头":  true,
		"网络-设置返回头": true,
		"网络-访问方法": true,
		"网络-权限":   true,
		"GPT-创建单轮对话": true,
		"GPT-增加文本":  true,
		"GPT-增加图片":  true,
		"GPT-发送请求":  true,
		"GPT-获取回复":  true,
		"GPT-删除指针":  true,
		"邮件主题":    true,
		"脚本输出":    true,
		"脚本输出-增加ID": true,
	}
}
