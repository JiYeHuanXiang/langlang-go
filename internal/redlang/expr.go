// Package redlang 提供 RedLang 表达式求值器。
// 支持 【计算】 命令所需的算术和逻辑表达式解析。
package redlang

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/scanner"
)

// ExprNode 表达式 AST 节点
type ExprNode struct {
	Type  ExprNodeType
	Value string       // 字面量
	Op    string       // 运算符
	Left  *ExprNode    // 二元左操作数
	Right *ExprNode    // 二元右操作数
	Child *ExprNode    // 一元操作数
}

type ExprNodeType int

const (
	ExprNumber ExprNodeType = iota
	ExprString
	ExprUnary
	ExprBinary
)

// 运算符优先级（数字越大优先级越高）
var prec = map[string]int{
	"||": 1,
	"&&": 2,
	"==": 3, "!=": 3, "<": 3, "<=": 3, ">": 3, ">=": 3,
	"+": 4, "-": 4,
	"*": 5, "/": 5, "%": 5, "//": 5,
	"^": 7,
}

// unaryPrec 一元运算符优先级（高于所有二元运算符，除 ^ 外）
const unaryPrec = 6

// 一元运算符
var unaryOps = map[string]bool{"!": true, "-": true}

// isDigit 判断符文是否为数字
func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// isAlpha 判断符文是否为字母或中文
func isAlpha(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch >= 0x80
}

// isIdent 判断符文是否为标识符字符
func isIdent(ch rune) bool {
	return isAlpha(ch) || isDigit(ch) || ch == '_' || ch == '.'
}

// ExprLexer 表达式词法分析器
type ExprLexer struct {
	input []rune
	pos   int
}

func NewExprLexer(input string) *ExprLexer {
	return &ExprLexer{input: []rune(input)}
}

// Token 表达式 Token
type Token struct {
	Type  TokenType
	Value string
}

type TokenType int

const (
	TkNumber TokenType = iota
	TkString
	TkIdent
	TkOp
	TkLParen
	TkRParen
	TkEOF
)

func (l *ExprLexer) skipWhitespace() {
	for l.pos < len(l.input) && (l.input[l.pos] == ' ' || l.input[l.pos] == '\t' || l.input[l.pos] == '\n' || l.input[l.pos] == '\r') {
		l.pos++
	}
}

func (l *ExprLexer) Next() Token {
	l.skipWhitespace()
	if l.pos >= len(l.input) {
		return Token{Type: TkEOF, Value: ""}
	}

	ch := l.input[l.pos]

	// 数字
	if isDigit(ch) || (ch == '.' && l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1])) {
		start := l.pos
		// 支持十六进制 0x
		if ch == '0' && l.pos+1 < len(l.input) && (l.input[l.pos+1] == 'x' || l.input[l.pos+1] == 'X') {
			l.pos += 2
			for l.pos < len(l.input) && isIdent(l.input[l.pos]) {
				l.pos++
			}
		} else {
			dotSeen := (ch == '.')
			l.pos++
			for l.pos < len(l.input) {
				nch := l.input[l.pos]
				if isDigit(nch) {
					l.pos++
				} else if nch == '.' && !dotSeen {
					dotSeen = true
					l.pos++
				} else {
					break
				}
			}
		}
		return Token{Type: TkNumber, Value: string(l.input[start:l.pos])}
	}

	// 字符串（被引号包围）
	if ch == '"' || ch == '\'' {
		quote := ch
		l.pos++ // 跳过引号
		start := l.pos
		for l.pos < len(l.input) && l.input[l.pos] != quote {
			if l.input[l.pos] == '\\' && l.pos+1 < len(l.input) {
				l.pos += 2
			} else {
				l.pos++
			}
		}
		val := string(l.input[start:l.pos])
		if l.pos < len(l.input) {
			l.pos++ // 跳过结束引号
		}
		return Token{Type: TkString, Value: val}
	}

	// 括号
	if ch == '(' {
		l.pos++
		return Token{Type: TkLParen, Value: "("}
	}
	if ch == ')' {
		l.pos++
		return Token{Type: TkRParen, Value: ")"}
	}

	// 运算符（多字符优先）
	ops := []string{"<=", ">=", "==", "!=", "||", "&&", "//", "^", "!", "+", "-", "*", "/", "%", "<", ">"}
	for _, op := range ops {
		opRunes := []rune(op)
		if l.pos+len(opRunes) <= len(l.input) {
			match := true
			for i, r := range opRunes {
				if l.input[l.pos+i] != r {
					match = false
					break
				}
			}
			if match {
				l.pos += len(opRunes)
				return Token{Type: TkOp, Value: op}
			}
		}
	}

	// 标识符（中文、字母开头）
	if isAlpha(ch) || ch == '_' {
		start := l.pos
		l.pos++
		for l.pos < len(l.input) && isIdent(l.input[l.pos]) {
			l.pos++
		}
		return Token{Type: TkIdent, Value: string(l.input[start:l.pos])}
	}

	// 单个特殊字符作普通文本
	l.pos++
	return Token{Type: TkString, Value: string(ch)}
}

// ExprParser 表达式解析器（递归下降 + 优先级爬升）
type ExprParser struct {
	lexer  *ExprLexer
	token  Token
	err    error
}

func NewExprParser(input string) *ExprParser {
	p := &ExprParser{lexer: NewExprLexer(input)}
	p.nextToken()
	return p
}

func (p *ExprParser) nextToken() {
	p.token = p.lexer.Next()
}

// Parse 解析整个表达式
func (p *ExprParser) Parse() (*ExprNode, error) {
	result := p.parseExpr(0)
	if p.err != nil {
		return nil, p.err
	}
	// 允许存在结尾的无关字符（容错）
	return result, nil
}

// parseExpr 用优先级爬升法解析表达式
func (p *ExprParser) parseExpr(minPrec int) *ExprNode {
	left := p.parsePrimary()
	if p.err != nil || left == nil {
		return left
	}

	for {
		if p.token.Type != TkOp {
			break
		}
		op := p.token.Value
		precLevel, isOp := prec[op]
		if !isOp || precLevel < minPrec {
			break
		}

		p.nextToken()

		// 右结合（^ 幂运算）
		nextMin := precLevel
		if op == "^" {
			nextMin = precLevel
		} else {
			nextMin = precLevel + 1
		}

		right := p.parseExpr(nextMin)
		if p.err != nil {
			return nil
		}

		left = &ExprNode{
			Type:  ExprBinary,
			Op:    op,
			Left:  left,
			Right: right,
		}
	}
	return left
}

// parsePrimary 解析基本表达式（数字、字符串、标识符、一元、括号）
func (p *ExprParser) parsePrimary() *ExprNode {
	switch p.token.Type {
	case TkNumber:
		node := &ExprNode{Type: ExprNumber, Value: p.token.Value}
		p.nextToken()
		return node

	case TkString:
		node := &ExprNode{Type: ExprString, Value: p.token.Value}
		p.nextToken()
		return node

	case TkIdent:
		// 将标识符视为字符串/变量引用，但这里作为字面量返回
		node := &ExprNode{Type: ExprString, Value: p.token.Value}
		p.nextToken()
		return node

	case TkLParen:
		p.nextToken() // 跳过 (
		node := p.parseExpr(0)
		if p.err != nil {
			return nil
		}
		if p.token.Type != TkRParen {
			p.err = fmt.Errorf("缺少右括号")
			return nil
		}
		p.nextToken() // 跳过 )
		return node

	case TkOp:
		op := p.token.Value
		if unaryOps[op] {
			p.nextToken()
			operand := p.parseExpr(unaryPrec)
			if p.err != nil {
				return nil
			}
			return &ExprNode{
				Type:  ExprUnary,
				Op:    op,
				Child: operand,
			}
		}
		// 不是一元运算符，回退
		p.err = fmt.Errorf("意外的运算符: %s", op)
		return nil

	default:
		p.err = fmt.Errorf("意外的 token: %v", p.token.Value)
		return nil
	}
}

// EvalExpr 对表达式求值，返回文本结果
// vars 提供变量查找函数（接收变量名返回文本值）
func EvalExpr(expr string, lookupVar func(name string) string) (string, error) {
	parser := NewExprParser(expr)
	node, err := parser.Parse()
	if err != nil {
		return "", fmt.Errorf("表达式解析错误: %w", err)
	}
	result, err := evalNode(node, lookupVar)
	if err != nil {
		return "", fmt.Errorf("表达式求值错误: %w", err)
	}
	return result, nil
}

// evalNode 递归求值表达式节点
func evalNode(node *ExprNode, lookupVar func(name string) string) (string, error) {
	if node == nil {
		return "", nil
	}

	switch node.Type {
	case ExprNumber:
		return node.Value, nil

	case ExprString:
		return node.Value, nil

	case ExprUnary:
		val, err := evalNode(node.Child, lookupVar)
		if err != nil {
			return "", err
		}
		switch node.Op {
		case "!":
			if isTrue(val) {
				return "假", nil
			}
			return "真", nil
		case "-":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return "", fmt.Errorf("取负操作数不是数字: %s", val)
			}
			return formatNum(-f), nil
		default:
			return "", fmt.Errorf("不支持的一元运算符: %s", node.Op)
		}

	case ExprBinary:
		leftStr, err := evalNode(node.Left, lookupVar)
		if err != nil {
			return "", err
		}
		rightStr, err := evalNode(node.Right, lookupVar)
		if err != nil {
			return "", err
		}

		switch node.Op {
		// 算术
		case "+":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			return formatNum(l + r), nil
		case "-":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			return formatNum(l - r), nil
		case "*":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			return formatNum(l * r), nil
		case "/":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			if r == 0 {
				return "0", nil
			}
			return formatNum(l / r), nil
		case "//":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			if r == 0 {
				return "0", nil
			}
			return formatInt(int64(l) / int64(r)), nil
		case "%":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			if r == 0 {
				return "0", nil
			}
			return formatInt(int64(l) % int64(r)), nil
		case "^":
			l, r, err := toNums(leftStr, rightStr)
			if err != nil {
				return "", err
			}
			return formatNum(math.Pow(l, r)), nil

		// 比较
		case "==":
			return boolStr(leftStr == rightStr), nil
		case "!=":
			return boolStr(leftStr != rightStr), nil
		case "<":
			return boolStr(compareNum(leftStr, rightStr) < 0), nil
		case "<=":
			return boolStr(compareNum(leftStr, rightStr) <= 0), nil
		case ">":
			return boolStr(compareNum(leftStr, rightStr) > 0), nil
		case ">=":
			return boolStr(compareNum(leftStr, rightStr) >= 0), nil

		// 逻辑
		case "&&":
			if isTrue(leftStr) && isTrue(rightStr) {
				return "真", nil
			}
			return "假", nil
		case "||":
			if isTrue(leftStr) || isTrue(rightStr) {
				return "真", nil
			}
			return "假", nil

		default:
			return "", fmt.Errorf("不支持的运算符: %s", node.Op)
		}

	default:
		return "", fmt.Errorf("未知节点类型: %v", node.Type)
	}
}

// 工具函数

func toNums(a, b string) (float64, float64, error) {
	fa, err := strconv.ParseFloat(a, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("不是有效数字: %s", a)
	}
	fb, err := strconv.ParseFloat(b, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("不是有效数字: %s", b)
	}
	return fa, fb, nil
}

// compareNum 数值比较（如果都能转成数字则按数字比较，否则按字符串比较）
func compareNum(a, b string) int {
	fa, errA := strconv.ParseFloat(a, 64)
	fb, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		if fa < fb {
			return -1
		}
		if fa > fb {
			return 1
		}
		return 0
	}
	return strings.Compare(a, b)
}

// formatNum 格式化数字，去掉多余的 .0
func formatNum(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return formatInt(int64(f))
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

func boolStr(b bool) string {
	if b {
		return "真"
	}
	return "假"
}

// isTrue 判断 RedLang 真值
func isTrue(s string) bool {
	return s == "真"
}

// 确保 scanner 不被编译器优化掉
var _ = scanner.EOF
