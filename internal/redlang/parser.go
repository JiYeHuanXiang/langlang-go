package redlang

import (
	"strings"
	"unicode"
)

// AstParser 是 RedLang 源码解析器
type AstParser struct {
	input []rune
	pos   int
}

// Parse 解析 RedLang 源码，返回 AST
func Parse(input string) (Ast, error) {
	p := &AstParser{
		input: []rune(input),
		pos:   0,
	}
	p.removeComment()
	return p.parse()
}

// ParseExpr 解析单个表达式
func ParseExpr(input string) (AstNode, error) {
	p := &AstParser{
		input: []rune(input),
		pos:   0,
	}
	nodes, err := p.parse()
	if err != nil {
		return AstNode{}, err
	}
	if len(nodes) == 0 {
		return AstNode{Type: TypeText, Text: ""}, nil
	}
	return nodes[0], nil
}

// removeComment 移除注释（从 `##` 到行尾，或从 `#` 到行尾，与原始 RedLang 规范一致）
// 注意：不处理 `//`，因为 `//` 在 【计算】 表达式中用作整除运算符
func (p *AstParser) removeComment() {
	var out []rune
	i := 0
	for i < len(p.input) {
		ch := p.input[i]
		if ch == '\\' && i+1 < len(p.input) {
			// 转义字符，原样保留
			out = append(out, ch, p.input[i+1])
			i += 2
			continue
		}
		// RedLang 注释：## 到行尾，或 # 到行尾
		if ch == '#' {
			// 跳过到行尾
			for i < len(p.input) && p.input[i] != '\n' {
				i++
			}
			continue
		}
		out = append(out, ch)
		i++
	}
	p.input = out
}

func (p *AstParser) parse() (Ast, error) {
	var nodes Ast
	for p.pos < len(p.input) {
		ch := p.input[p.pos]

		if ch == '【' {
			cmd, err := p.parseCommand()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, AstNode{Type: TypeCommand, Cmd: cmd})
		} else if ch == '\\' && p.pos+1 < len(p.input) {
			// 转义字符 -> 作为文本
			p.pos++
			nodes = append(nodes, AstNode{Type: TypeText, Text: string(p.input[p.pos])})
			p.pos++
		} else {
			// 普通文本
			text := p.parseText()
			nodes = append(nodes, AstNode{Type: TypeText, Text: text})
		}
	}
	return nodes, nil
}

// parseText 解析直到遇到命令开始符或结束
func (p *AstParser) parseText() string {
	var b strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '【' || ch == '】' {
			break
		}
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			b.WriteRune(p.input[p.pos])
			p.pos++
			continue
		}
		b.WriteRune(ch)
		p.pos++
	}
	// 修剪首尾空白
	s := b.String()
	s = strings.TrimLeftFunc(s, unicode.IsSpace)
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	return s
}

// parseCommand 解析 【命令名】@参数...】
func (p *AstParser) parseCommand() (*AstCmd, error) {
	// 跳过 【
	p.pos++

	// 解析命令名（遇到 @ 或 】 结束）
	var name strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '@' || ch == '】' {
			break
		}
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			name.WriteRune(p.input[p.pos])
			p.pos++
			continue
		}
		name.WriteRune(ch)
		p.pos++
	}

	cmd := &AstCmd{
		Name: strings.TrimSpace(name.String()),
		Args: nil,
	}

	// 如果名字以 】 结束，跳过它（处理 【名字】@arg 语法）
	if p.pos < len(p.input) && p.input[p.pos] == '】' {
		p.pos++
	}

	// 判断是否是原始函数体命令（需要捕获原始文本）
	rawBodyCmds := map[string]bool{
		"函数定义": true,
	}

	// 解析参数：连续 @arg1@arg2...
	// 每个参数要么是一个嵌套命令（@【命令】），要么是纯文本（@文本）
	for p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++ // 跳过 @

		if p.pos >= len(p.input) {
			break
		}

		// 如果是原始函数体命令，捕获所有内容直到匹配的 】
		if rawBodyCmds[cmd.Name] {
			depth := 1
			start := p.pos
			for p.pos < len(p.input) && depth > 0 {
				ch := p.input[p.pos]
				if ch == '【' {
					depth++
				} else if ch == '】' {
					depth--
					if depth == 0 {
						break
					}
				}
				p.pos++
			}
			rawBody := string(p.input[start:p.pos])
			rawBody = strings.TrimSpace(rawBody)
			cmd.RawBody = rawBody
			// 跳过 】
			if p.pos < len(p.input) && p.input[p.pos] == '】' {
				p.pos++
			}
			return cmd, nil
		}

		var argAst Ast
		if p.input[p.pos] == '【' {
			// 嵌套命令参数
			subCmd, err := p.parseCommand()
			if err != nil {
				return nil, err
			}
			argAst = append(argAst, AstNode{Type: TypeCommand, Cmd: subCmd})
		} else if p.input[p.pos] == '\\' && p.pos+1 < len(p.input) {
			p.pos++
			argAst = append(argAst, AstNode{Type: TypeText, Text: string(p.input[p.pos])})
			p.pos++
		} else {
			// 纯文本参数 — 读到下一个 @、】或顶层 【 为止
			var sb strings.Builder
			for p.pos < len(p.input) && p.input[p.pos] != '@' && p.input[p.pos] != '】' && p.input[p.pos] != '【' {
				if p.input[p.pos] == '\\' && p.pos+1 < len(p.input) {
					p.pos++
					sb.WriteRune(p.input[p.pos])
					p.pos++
					continue
				}
				sb.WriteRune(p.input[p.pos])
				p.pos++
			}
			argAst = append(argAst, AstNode{Type: TypeText, Text: strings.TrimSpace(sb.String())})
		}
		if len(argAst) > 0 {
			cmd.Args = append(cmd.Args, argAst)
		}
	}

	// 跳过 】
	if p.pos < len(p.input) && p.input[p.pos] == '】' {
		p.pos++
	}

	return cmd, nil
}
