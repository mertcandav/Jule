package parser

import (
	"github.com/the-xlang/x/ast"
	"github.com/the-xlang/x/lex"
	"github.com/the-xlang/x/pkg/x"
)

func typeIsVoidReturn(t ast.DataTypeAST) bool {
	return t.Code == x.Void && !t.MultiTyped
}

func typeOfArrayElements(t ast.DataTypeAST) ast.DataTypeAST {
	// Remove array syntax "[]"
	t.Value = t.Value[2:]
	return t
}

func typeIsPointer(t ast.DataTypeAST) bool {
	if t.Value == "" {
		return false
	}
	return t.Value[0] == '*'
}

func typeIsArray(t ast.DataTypeAST) bool {
	if t.Value == "" {
		return false
	}
	return t.Value[0] == '['
}

func typeIsSingle(dt ast.DataTypeAST) bool {
	return !typeIsPointer(dt) &&
		!typeIsArray(dt) &&
		dt.Code != x.Function
}

func checkValidityConstantDataType(dt ast.DataTypeAST) bool {
	return typeIsSingle(dt)
}

func (p *Parser) checkValidityForAutoType(t ast.DataTypeAST, err lex.Token) {
	switch t.Code {
	case x.Nil:
		p.PushErrorToken(err, "nil_for_autotype")
	case x.Void:
		p.PushErrorToken(err, "void_for_autotype")
	}
}

func (p *Parser) defaultValueOfType(t ast.DataTypeAST) string {
	if typeIsPointer(t) || typeIsArray(t) {
		return "nil"
	}
	return x.DefaultValueOfType(t.Code)
}

func typeIsNilCompatible(t ast.DataTypeAST) bool {
	return t.Code == x.Function || typeIsPointer(t)
}

func checkArrayCompatiblity(arrT, t ast.DataTypeAST) bool {
	if t.Code == x.Nil {
		return true
	}
	return arrT.Value == t.Value
}

func typesAreCompatible(t1, t2 ast.DataTypeAST, ignoreany bool) bool {
	switch {
	case typeIsArray(t1) || typeIsArray(t2):
		if typeIsArray(t2) {
			t1, t2 = t2, t1
		}
		return checkArrayCompatiblity(t1, t2)
	case typeIsNilCompatible(t1) || typeIsNilCompatible(t2):
		return t1.Code == x.Nil || t2.Code == x.Nil
	}
	return x.TypesAreCompatible(t1.Code, t2.Code, ignoreany)
}

func (p *Parser) readyType(dt ast.DataTypeAST) (ast.DataTypeAST, bool) {
	if dt.Value == "" {
		return dt, true
	}
	switch dt.Code {
	case x.Name:
		t := p.typeByName(dt.Token.Kind)
		if t == nil {
			return dt, false
		}
		t.Type.Value = dt.Value[:len(dt.Value)-len(dt.Token.Kind)] + t.Type.Value
		return p.readyType(t.Type)
	case x.Function:
		funAST := dt.Tag.(ast.FunctionAST)
		for index, param := range funAST.Params {
			funAST.Params[index].Type, _ = p.readyType(param.Type)
		}
		funAST.ReturnType, _ = p.readyType(funAST.ReturnType)
		dt.Value = dt.Tag.(ast.FunctionAST).DataTypeString()
	}
	return dt, true
}

func (p *Parser) checkMultiType(real, check ast.DataTypeAST, ignoreAny bool, errToken lex.Token) {
	if real.MultiTyped != check.MultiTyped {
		p.PushErrorToken(errToken, "incompatible_datatype")
		return
	}
	realTypes := real.Tag.([]ast.DataTypeAST)
	checkTypes := real.Tag.([]ast.DataTypeAST)
	if len(realTypes) != len(checkTypes) {
		p.PushErrorToken(errToken, "incompatible_datatype")
		return
	}
	for index := 0; index < len(realTypes); index++ {
		realType := realTypes[index]
		checkType := checkTypes[index]
		p.checkType(realType, checkType, ignoreAny, errToken)
	}
}

func (p *Parser) checkType(real, check ast.DataTypeAST, ignoreAny bool, errToken lex.Token) {
	real, ok := p.readyType(real)
	if !ok {
		return
	}
	check, ok = p.readyType(check)
	if !ok {
		return
	}
	if !ignoreAny && real.Code == x.Any {
		return
	}
	if real.MultiTyped || check.MultiTyped {
		p.checkMultiType(real, check, ignoreAny, errToken)
		return
	}
	if typeIsSingle(real) && typeIsSingle(check) {
		if !x.TypesAreCompatible(check.Code, real.Code, ignoreAny) {
			p.PushErrorToken(errToken, "incompatible_datatype")
		}
		return
	}
	if (typeIsPointer(real) || typeIsArray(real)) &&
		check.Code == x.Nil {
		return
	}
	if real.Value != check.Value {
		p.PushErrorToken(errToken, "incompatible_datatype")
	}
}
