package models

import "strings"

// Case the AST model of case.
type Case struct {
	Exprs []Expr
	Block Block
}

func (c *Case) String(matchExpr string) string {
	var cxx strings.Builder
	if len(c.Exprs) > 0 {
		cxx.WriteString("if (")
		for i, expr := range c.Exprs {
			cxx.WriteString(expr.String())
			if matchExpr != "" {
				cxx.WriteString(" == ")
				cxx.WriteString(matchExpr)
			}
			if i+1 < len(c.Exprs) {
				cxx.WriteString(" || ")
			}
		}
		cxx.WriteByte(')')
	}
	cxx.WriteString(" { do ")
	cxx.WriteString(c.Block.String())
	cxx.WriteString("while(false);")
	if len(c.Exprs) > 0 {
		cxx.WriteByte('}')
	}
	return cxx.String()
}

// Match the AST model of match-case.
type Match struct {
	Tok      Tok
	Expr     Expr
	ExprType DataType
	Default  *Case
	Cases    []Case
}

func (m *Match) MatchExprString() string {
	if len(m.Cases) == 0 {
		if m.Default != nil {
			return m.Default.String("")
		}
		return ""
	}
	var cxx strings.Builder
	cxx.WriteString("{\n")
	AddIndent()
	cxx.WriteString(IndentString())
	cxx.WriteString(m.ExprType.String())
	cxx.WriteString(" &&expr{")
	cxx.WriteString(m.Expr.String())
	cxx.WriteString("};\n")
	cxx.WriteString(IndentString())
	cxx.WriteString(m.Cases[0].String("expr"))
	for _, c := range m.Cases[1:] {
		cxx.WriteString("else ")
		cxx.WriteString(c.String("expr"))
	}
	if m.Default != nil {
		cxx.WriteString("else ")
		cxx.WriteString(m.Default.String(""))
		cxx.WriteByte('}')
	}
	cxx.WriteByte('\n')
	DoneIndent()
	cxx.WriteString(IndentString())
	cxx.WriteByte('}')
	return cxx.String()
}

func (m *Match) MatchBoolString() string {
	var cxx strings.Builder
	cxx.WriteString(m.Cases[0].String(""))
	for _, c := range m.Cases[1:] {
		cxx.WriteString("else ")
		cxx.WriteString(c.String(""))
	}
	if m.Default != nil {
		cxx.WriteString("else ")
		cxx.WriteString(m.Default.String(""))
		cxx.WriteByte('}')
	}
	return cxx.String()
}

func (m Match) String() string {
	if m.Expr.Model != nil {
		return m.MatchExprString()
	}
	return m.MatchBoolString()
}
