package sema

import (
	"strconv"
	"strings"

	"github.com/julelang/jule/ast"
	"github.com/julelang/jule/constant/lit"
	"github.com/julelang/jule/lex"
	"github.com/julelang/jule/types"
)

// Value data.
type Data struct {
	Kind       *TypeKind
	Mutable    bool
	Lvalue     bool
	Variadiced bool

	// True if kind is declaration such as:
	//  - *Enum
	//  - *Struct
	Decl       bool

	// This field is reminder.
	// Will write to every constant processing points.
	// Changed after add constant evaluation support.
	// So, reminder flag for constants.
	Constant   bool
}

// Reports whether Data is nil literal.
func (d *Data) Is_nil() bool { return d.Kind.Is_nil() }
// Reports whether Data is void.
func (d *Data) Is_void() bool { return d.Kind == nil }

func build_void_data() *Data {
	return &Data{
		Mutable:  false,
		Lvalue:   false,
		Decl:     false,
		Constant: false,
		Kind:     nil,
	}
}

// Value.
type Value struct {
	Expr *ast.Expr
	Data *Data
}

func kind_by_bitsize(expr any) string {
	switch expr.(type) {
	case float64:
		x := expr.(float64)
		return types.Float_from_bits(types.Bitsize_of_float(x))

	case int64:
		x := expr.(int64)
		return types.Int_from_bits(types.Bitsize_of_int(x))

	case uint64:
		x := expr.(uint64)
		return types.Uint_from_bits(types.Bitsize_of_uint(x))

	default:
		return ""
	}
}

func check_data_for_integer_indexing(d *Data) (err_key string) {
	switch {
	case d.Kind.Prim() == nil:
		return "invalid_expr"

	case !types.Is_int(d.Kind.Prim().To_str()):
		return "invalid_expr"

	case d.Constant && false /* TODO: Check negative constants */:
		return "overflow_limits"

	default:
		return ""
	}
}

// Evaluator.
type _Eval struct {
	s        *_Sema  // Used for error logging.
	lookup   _Lookup
	prefix   *TypeKind
	unsafety bool
}

func (e *_Eval) push_err(token lex.Token, key string, args ...any) {
	e.s.errors = append(e.s.errors, compiler_err(token, key, args...))
}

// Reports whether evaluation in unsafe scope.
func (e *_Eval) is_unsafe() bool { return e.unsafety }

func (e *_Eval) lit_nil() *Data {
	// Return new Data with nil kind.
	// Nil kind represents "nil" literal.
	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: true,
		Decl:     false,
		Kind:     &TypeKind{kind: nil},
	}
}

func (e *_Eval) lit_str(lit *ast.LitExpr) *Data {
	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: true,
		Decl:     false,
		Kind:     &TypeKind{
			kind: build_prim_type(types.TypeKind_STR),
		},
	}
}

func (e *_Eval) lit_bool(lit *ast.LitExpr) *Data {
	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: true,
		Decl:     false,
		Kind:     &TypeKind{
			kind: build_prim_type(types.TypeKind_BOOL),
		},
	}
}

func (e *_Eval) lit_rune(l *ast.LitExpr) *Data {
	const BYTE_KIND = types.TypeKind_U8
	const RUNE_KIND = types.TypeKind_I32
	
	data := &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: true,
		Decl:     false,
	}

	_, is_byte := lit.Is_byte_lit(l.Value)
	if is_byte {
		data.Kind = &TypeKind{
			kind: build_prim_type(BYTE_KIND),
		}
	} else {
		data.Kind = &TypeKind{
			kind: build_prim_type(RUNE_KIND),
		}
	}

	return data
}

func (e *_Eval) lit_float(l *ast.LitExpr) *Data {
	const FLOAT_KIND = types.TypeKind_F64

	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: true,
		Decl:     false,
		Kind:     &TypeKind{
			kind: build_prim_type(FLOAT_KIND),
		},
	}
}

func (e *_Eval) lit_int(l *ast.LitExpr) *Data {
	data := l.Value
	base := 0

	switch {
	case strings.HasPrefix(data, "0x"): // Hexadecimal
		data = data[2:]
		base = 0b00010000

		case strings.HasPrefix(data, "0b"): // Binary
		data = data[2:]
		base = 0b10

	case data[0] == '0' && len(data) > 1: // Octal
		data = data[1:]
		base = 0b1000

	default: // Decimal
		base = 0b1010
	}

	var value any = nil
	const BIT_SIZE = 0b01000000
	temp_value, err := strconv.ParseInt(data, base, BIT_SIZE)
	if err == nil {
		value = temp_value
	} else {
		value, _ = strconv.ParseUint(data, base, BIT_SIZE)
	}

	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: true,
		Decl:     false,
		Kind:     &TypeKind{
			kind: build_prim_type(kind_by_bitsize(value)),
		},
	}
}

func (e *_Eval) lit_num(l *ast.LitExpr) *Data {
	switch {
	case lex.Is_float(l.Value):
		return e.lit_float(l)

	default:
		return e.lit_int(l)
	}
}

func (e *_Eval) eval_lit(lit *ast.LitExpr) *Data {
	switch {
	case lit.Is_nil():
		return e.lit_nil()

	case lex.Is_str(lit.Value):
		return e.lit_str(lit)

	case lex.Is_bool(lit.Value):
		return e.lit_bool(lit)

	case lex.Is_rune(lit.Value):
		return e.lit_rune(lit)

	case lex.Is_num(lit.Value):
		return e.lit_num(lit)

	default:
		return nil
	}
}

func (e *_Eval) get_def(ident string, cpp_linked bool) any {
	if !cpp_linked {
		enm := e.lookup.find_enum(ident)
		if enm != nil {
			return enm
		}
	}

	v := e.lookup.find_var(ident, cpp_linked)
	if v != nil {
		return v
	}

	s := e.lookup.find_struct(ident, cpp_linked)
	if s != nil {
		return s
	}

	ta := e.lookup.find_type_alias(ident, cpp_linked)
	if ta != nil {
		return ta
	}

	return nil
}

func (e *_Eval) eval_enum(enm *Enum, error_token lex.Token) *Data {
	if !e.s.is_accessible_define(enm.Public, enm.Token) {
		e.push_err(error_token, "ident_not_exist", enm.Ident)
		return nil
	}

	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: false,
		Decl:     true,
		Kind:     &TypeKind{
			kind: enm,
		},
	}
}

func (e *_Eval) eval_struct(s *StructIns, error_token lex.Token) *Data {
	if !e.s.is_accessible_define(s.Decl.Public, s.Decl.Token) {
		e.push_err(error_token, "ident_not_exist", s.Decl.Ident)
		return nil
	}

	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: false,
		Decl:     true,
		Kind:     &TypeKind{
			kind: s,
		},
	}
}

func (e *_Eval) eval_fn(f *Fn, error_token lex.Token) *Data {
	if !e.s.is_accessible_define(f.Public, f.Token) {
		e.push_err(error_token, "ident_not_exist", f.Ident)
		return nil
	}

	return &Data{
		Lvalue:   false,
		Mutable:  false,
		Constant: false,
		Decl:     false,
		Kind:     &TypeKind{
			kind: f.instance(),
		},
	}
}

func (e *_Eval) eval_var(v *Var, error_token lex.Token) *Data {
	if !e.s.is_accessible_define(v.Public, v.Token) {
		e.push_err(error_token, "ident_not_exist", v.Ident)
		return nil
	}

	return &Data{
		Lvalue:   !v.Constant,
		Mutable:  v.Mutable,
		Constant: v.Constant,
		Decl:     false,
		Kind:     v.Kind.Kind,
	}
}

func (e *_Eval) eval_type_alias(ta *TypeAlias, error_token lex.Token) *Data {
	if !e.s.is_accessible_define(ta.Public, ta.Token) {
		e.push_err(error_token, "ident_not_exist", ta.Ident)
		return nil
	}

	kind := ta.Kind.Kind.kind
	switch kind.(type) {
	case *StructIns:
		return e.eval_struct(kind.(*StructIns), error_token)

	case *Enum:
		return e.eval_enum(kind.(*Enum), error_token)

	default:
		e.push_err(error_token, "invalid_expr")
		return nil
	}
}

func (e *_Eval) eval_def(def any, ident lex.Token) *Data {
	switch def.(type) {
	case *Var:
		return e.eval_var(def.(*Var), ident)

	case *Enum:
		return e.eval_enum(def.(*Enum), ident)

	case *Struct:
		return e.eval_struct(def.(*Struct).instance(), ident)

	case *Fn:
		return e.eval_fn(def.(*Fn), ident)

	case *TypeAlias:
		return e.eval_type_alias(def.(*TypeAlias), ident)

	default:
		e.push_err(ident, "ident_not_exist", ident.Kind)
		return nil
	}
}

func (e *_Eval) eval_ident(ident *ast.IdentExpr) *Data {
	def := e.get_def(ident.Ident, ident.Cpp_linked)
	return e.eval_def(def, ident.Token)
}

func (e *_Eval) eval_unary_minus(d *Data) *Data {
	t := d.Kind.Prim()
	if t == nil || !types.Is_num(t.To_str()) {
		return nil
	}
	// TODO: Eval constants.
	// TODO: Check out d.Lvalue should be false?
	return d
}

func (e *_Eval) eval_unary_plus(d *Data) *Data {
	t := d.Kind.Prim()
	if t == nil || !types.Is_num(t.To_str()) {
		return nil
	}
	// TODO: Eval constants.
	// TODO: Check out d.Lvalue should be false?
	return d
}

func (e *_Eval) eval_unary_caret(d *Data) *Data {
	t := d.Kind.Prim()
	if t == nil || !types.Is_int(t.To_str()) {
		return nil
	}
	// TODO: Eval constants.
	// TODO: Check out d.Lvalue should be false?
	return d
}

func (e *_Eval) eval_unary_excl(d *Data) *Data {
	t := d.Kind.Prim()
	if t == nil || !t.Is_bool() {
		return nil
	}
	// TODO: Eval constants.
	// TODO: Check out d.Lvalue should be false?
	return d
}

func (e *_Eval) eval_unary_star(d *Data, op lex.Token) *Data {
	if !e.is_unsafe() {
		e.push_err(op, "unsafe_behavior_at_out_of_unsafe_scope")
	}

	t := d.Kind.Ptr()
	if t == nil || t.Is_unsafe() {
		return nil
	}
	d.Constant = false
	d.Lvalue = true
	return d
}

func (e *_Eval) eval_unary_amper(d *Data) *Data {
	switch {
	case d.Kind.Ref() != nil:
		// Ok

	case !can_get_ptr(d):
		d = nil
	}

	if d != nil {
		d.Constant = false
		d.Lvalue = true
		d.Mutable = true
	}

	return d
}

func (e *_Eval) eval_unary(u *ast.UnaryExpr) *Data {
	data := e.eval_expr_kind(u.Expr)
	if data == nil {
		return nil
	}

	switch u.Op.Kind {
	case lex.KND_MINUS:
		data = e.eval_unary_minus(data)

	case lex.KND_PLUS:
		data = e.eval_unary_plus(data)

	case lex.KND_CARET:
		data = e.eval_unary_caret(data)

	case lex.KND_EXCL:
		data = e.eval_unary_excl(data)

	case lex.KND_STAR:
		data = e.eval_unary_star(data, u.Op)

	case lex.KND_AMPER:
		data = e.eval_unary_amper(data)

	default:
		data = nil
	}

	if data == nil {
		e.push_err(u.Op, "invalid_expr_unary_operator", u.Op.Kind)
	}

	return data
}

func (e *_Eval) eval_variadic(v *ast.VariadicExpr) *Data {
	d := e.eval_expr_kind(v.Expr)
	if d == nil {
		return nil
	}

	if !is_variadicable(d.Kind) {
		e.push_err(v.Token, "variadic_with_non_variadicable", d.Kind.To_str())
		return nil
	}

	d.Variadiced = true
	return d
}

func (e *_Eval) eval_unsafe(u *ast.UnsafeExpr) *Data {
	unsafety := e.unsafety
	e.unsafety = true

	d := e.eval_expr_kind(u.Expr)

	e.unsafety = unsafety

	return d
}

func (e *_Eval) eval_arr(s *ast.SliceExpr) *Data {
	// Arrays always has type prefixes.
	pt := e.prefix.Arr()

	arr := &Arr{
		Auto: false,
		N:    0,
		Elem: pt.Elem,
	}

	if pt.Auto {
		arr.N = len(s.Elems)
	} else {
		arr.N = len(s.Elems)
		if arr.N > pt.N {
			e.push_err(s.Token, "overflow_limits")
		}
	}

	for _, elem := range s.Elems {
		d := e.eval_expr_kind(elem)
		if d == nil {
			continue
		}

		e.s.check_assign_type(arr.Elem, d, s.Token, true)
	}

	return &Data{
		Kind: &TypeKind{kind: arr},
	}
}

func (e *_Eval) eval_exp_slc(s *ast.SliceExpr, elem_type *TypeKind) *Data {
	slc := &Slc{
		Elem: elem_type,
	}

	for _, elem := range s.Elems {
		d := e.eval_expr_kind(elem)
		if d == nil {
			continue
		}

		e.s.check_assign_type(slc.Elem, d, s.Token, true)
	}

	return &Data{
		Kind: &TypeKind{kind: slc},
	}
}

func (e *_Eval) eval_slice_expr(s *ast.SliceExpr) *Data {
	if e.prefix != nil {
		switch {
		case e.prefix.Arr() != nil:
			return e.eval_arr(s)

		case e.prefix.Slc() != nil:
			pt := e.prefix.Slc()
			return e.eval_exp_slc(s, pt.Elem)
		}
	}

	prefix := e.prefix
	e.prefix = nil

	if len(s.Elems) == 0 {
		e.push_err(s.Token, "dynamic_type_annotation_failed")
		return nil
	}

	first_elem := e.eval_expr_kind(s.Elems[0])
	if first_elem == nil {
		return nil
	}

	// Remove first element.
	// First element always compatible with element type
	// because first element determines to Slc's element type.
	s.Elems = s.Elems[1:]
	d := e.eval_exp_slc(s, first_elem.Kind)

	e.prefix = prefix
	return d
}

func (e *_Eval) check_integer_indexing_by_data(d *Data, token lex.Token) {
	err_key := check_data_for_integer_indexing(d)
	if err_key != "" {
		e.push_err(token, err_key)
	}
}

func (e *_Eval) check_integer_indexing(i *ast.IndexingExpr) {
	d := e.eval_expr_kind(i.Index)
	if d != nil {
		e.check_integer_indexing_by_data(d, i.Token)
	}
}

func (e *_Eval) indexing_ptr(d *Data, i *ast.IndexingExpr) {
	e.check_integer_indexing(i)

	ptr := d.Kind.Ptr()
	switch {
	case ptr.Is_unsafe():
		e.push_err(i.Token, "unsafe_ptr_indexing")
		return

	case !e.is_unsafe():
		e.push_err(i.Token, "unsafe_behavior_at_out_of_unsafe_scope")
	}

	d.Kind = ptr.Elem
}

func (e *_Eval) indexing_arr(d *Data, i *ast.IndexingExpr) {
	arr := d.Kind.Arr()
	d.Kind = arr.Elem
	e.check_integer_indexing(i)
}

func (e *_Eval) indexing_slc(d *Data, i *ast.IndexingExpr) {
	slc := d.Kind.Slc()
	d.Kind = slc.Elem
	e.check_integer_indexing(i)
}

func (e *_Eval) indexing_map(d *Data, i *ast.IndexingExpr) {
	m := d.Kind.Map()
	d.Kind = m.Val
	
	// TODO: Check element type compatibility.
}

func (e *_Eval) indexing_str(d *Data, i *ast.IndexingExpr) {
	const BYTE_KIND = types.TypeKind_U8
	d.Kind.kind = build_prim_type(BYTE_KIND)
	
	index := e.eval_expr_kind(i.Index)
	if index == nil {
		return
	}

	e.check_integer_indexing_by_data(index, i.Token)

	if !index.Constant {
		d.Constant = false
		return
	}

	if d.Constant {
		// TODO: Eval constant byte.
	}
}

func (e *_Eval) to_indexing(d *Data, i *ast.IndexingExpr) {
	switch {
	case d.Kind.Ptr() != nil:
		e.indexing_ptr(d, i)
		return

	case d.Kind.Arr() != nil:
		e.indexing_arr(d, i)
		return

	case d.Kind.Slc() != nil:
		e.indexing_slc(d, i)
		return

	case d.Kind.Map() != nil:
		e.indexing_map(d, i)
		return

	case d.Kind.Prim() != nil:
		prim := d.Kind.Prim()
		switch {
		case prim.Is_str():
			e.indexing_str(d, i)
			return
		}
	}

	e.push_err(i.Token, "not_supports_indexing", d.Kind.To_str())
}

func (e *_Eval) eval_indexing(i *ast.IndexingExpr) *Data {
	d := e.eval_expr_kind(i.Expr)
	if d == nil {
		return nil
	}

	e.to_indexing(d, i)
	return d
}

func (e *_Eval) eval_slicing_exprs(s *ast.SlicingExpr) (*Data, *Data) {
	var l *Data = nil
	var r *Data = nil

	if s.Start != nil {
		l = e.eval_expr_kind(s.Start)
		if l != nil {
			e.check_integer_indexing_by_data(l, s.Token)
		}
	} else {
		l = &Data{
			Constant: true,
			Kind: &TypeKind{kind: build_prim_type(types.SYS_INT)},
		}
	}

	if s.To != nil {
		r = e.eval_expr_kind(s.To)
		if r != nil {
			e.check_integer_indexing_by_data(r, s.Token)
		}
	}

	return l, r
}

func (e *_Eval) slicing_arr(d *Data, s *ast.SlicingExpr) {
	_, _ = e.eval_slicing_exprs(s)

	d.Lvalue = false
	d.Kind.kind = &Slc{Elem: d.Kind.Slc().Elem}
}

func (e *_Eval) slicing_slc(d *Data, s *ast.SlicingExpr) {
	_, _ = e.eval_slicing_exprs(s)

	d.Lvalue = false
}

func (e *_Eval) slicing_str(d *Data, s *ast.SlicingExpr) {
	d.Lvalue = false
	if !d.Constant {
		return
	}

	l, r := e.eval_slicing_exprs(s)
	if l == nil {
		return
	}
	_ = r // Ignore compiler error.

	// TODO: Eval constant string slicing.
}

func (e *_Eval) check_slicing(d *Data, s *ast.SlicingExpr) {
	switch {
	case d.Kind.Arr() != nil:
		e.slicing_arr(d, s)
		return

	case d.Kind.Slc() != nil:
		e.slicing_slc(d, s)
		return

	case d.Kind.Prim() != nil:
		prim := d.Kind.Prim()
		switch {
		case prim.Is_str():
			e.slicing_str(d, s)
			return
		}
	}

	e.push_err(s.Token, "not_supports_slicing", d.Kind.To_str())
}

func (e *_Eval) eval_slicing(s *ast.SlicingExpr) *Data {
	d := e.eval_expr_kind(s.Expr)
	if d == nil {
		return nil
	}

	e.check_slicing(d, s)
	return d
}

func (e *_Eval) cast_ptr(t *TypeKind, d *Data, error_token lex.Token) {
	if !e.is_unsafe() {
		e.push_err(error_token, "unsafe_behavior_at_out_of_unsafe_scope")
		return
	}

	prim := d.Kind.Prim()
	if d.Kind.Ptr() == nil && (prim == nil || !types.Is_int(prim.To_str())) {
		e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
	}

	d.Constant = false
}

func (e *_Eval) cast_struct(t *TypeKind, d *Data, error_token lex.Token) {
	tr := d.Kind.Trt()
	if tr == nil {
		e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
		return
	}

	s := t.Strct()
	if !s.Decl.Is_implements(tr) {
		e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
	}
}

func (e *_Eval) cast_ref(t *TypeKind, d *Data, error_token lex.Token) {
	ref := t.Ref()
	if ref.Elem.Strct() != nil {
		e.cast_struct(t, d, error_token)
		return
	}

	e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
}

func (e *_Eval) cast_slc(t *TypeKind, d *Data, error_token lex.Token) {
	if d.Kind.Prim() == nil || !d.Kind.Prim().Is_str() {
		e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
		return
	}

	t = t.Slc().Elem
	prim := t.Prim()
	if prim == nil || (!prim.Is_u8() && !prim.Is_i32()) {
		e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
	}
}

func (e *_Eval) cast_str(d *Data, error_token lex.Token) {
	if d.Kind.Prim() != nil {
		prim := d.Kind.Prim()
		if !prim.Is_u8() && !prim.Is_i32() {
			e.push_err(error_token, "type_not_supports_casting_to", types.TypeKind_STR, d.Kind.To_str())
		}
		return
	}

	if d.Kind.Slc() == nil {
		e.push_err(error_token, "type_not_supports_casting_to", types.TypeKind_STR, d.Kind.To_str())
		return
	}

	t := d.Kind.Slc().Elem
	prim := t.Prim()
	if prim == nil || (!prim.Is_u8() && !prim.Is_i32()) {
		e.push_err(error_token, "type_not_supports_casting_to", types.TypeKind_STR, d.Kind.To_str())
	}
}

func (e *_Eval) cast_int(t *TypeKind, d *Data, error_token lex.Token) {
	// TODO: Eval constant casting.

	if d.Kind.Enm() != nil {
		e := d.Kind.Enm()
		if types.Is_num(e.Kind.Kind.To_str()) {
			return
		}
	}

	if d.Kind.Ptr() != nil {
		prim := t.Prim()
		if prim.Is_uintptr() {
			// Ignore case.
		} else if !e.is_unsafe() {
			e.push_err(error_token, "unsafe_behavior_at_out_of_unsafe_scope")
		} else if !prim.Is_i32() && !prim.Is_i64() && !prim.Is_u16() && !prim.Is_u32() && !prim.Is_u64() {
			e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
		}
		return
	}

	prim := d.Kind.Prim()
	if prim != nil && types.Is_num(prim.To_str()) {
		return
	}

	e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
}

func (e *_Eval) cast_num(t *TypeKind, d *Data, error_token lex.Token) {
	// TODO: Eval constant casting.

	if d.Kind.Enm() != nil {
		e := d.Kind.Enm()
		if types.Is_num(e.Kind.Kind.To_str()) {
			return
		}
	}

	prim := d.Kind.Prim()
	if prim != nil && types.Is_num(prim.To_str()) {
		return
	}

	e.push_err(error_token, "type_not_supports_casting_to", d.Kind.To_str(), t.To_str())
}

func (e *_Eval) cast_prim(t *TypeKind, d *Data, error_token lex.Token) {
	prim := t.Prim()
	switch {
	case prim.Is_any():
		// The any type supports casting to any data type.

	case prim.Is_str():
		e.cast_str(d, error_token)

	case types.Is_int(prim.To_str()):
		e.cast_int(t, d, error_token)

	case types.Is_num(prim.To_str()):
		e.cast_num(t, d, error_token)

	default:
		e.push_err(error_token, "type_not_supports_casting", t.To_str())
	}
}

func (e *_Eval) eval_cast(c *ast.CastExpr) *Data {
	t := build_type(c.Kind)
	ok := e.s.check_type(t)
	if !ok {
		return nil
	}
	
	d := e.eval_expr_kind(c.Expr)
	if d == nil {
		return nil
	}

	switch {
	case t.Kind.Ptr() != nil:
		e.cast_ptr(t.Kind, d, c.Kind.Token)

	case t.Kind.Ref() != nil:
		e.cast_ref(t.Kind, d, c.Kind.Token)

	case t.Kind.Slc() != nil:
		e.cast_slc(t.Kind, d, c.Kind.Token)

	case t.Kind.Strct() != nil:
		e.cast_struct(t.Kind, d, c.Kind.Token)

	case t.Kind.Prim() != nil:
		e.cast_prim(t.Kind, d, c.Kind.Token)

	default:
		e.push_err(c.Kind.Token, "type_not_supports_casting", t.Kind.To_str())
	}

	d.Lvalue = is_lvalue(t.Kind)
	d.Mutable = is_mut(t.Kind)
	return d
}

func (e *_Eval) eval_ns_selection(s *ast.NsSelectionExpr) *Data {
	path := build_link_path_by_tokens(s.Ns)
	pkg := e.lookup.select_package(func(p *Package) bool {
		return p.Link_path == path
	})

	if pkg == nil {
		e.push_err(s.Ident, "namespace_not_exist", s.Ident.Kind)
		return nil
	}

	lookup := e.lookup
	e.lookup = pkg

	const CPP_LINKED = false
	def := e.get_def(s.Ident.Kind, CPP_LINKED)
	d := e.eval_def(def, s.Ident)

	e.lookup = lookup

	return d
}

func (e *_Eval) eval_struct_lit(lit *ast.StructLit) *Data {
	t := build_type(lit.Kind)
	ok := e.s.check_type(t)
	if !ok {
		return nil
	}

	s := t.Kind.Strct()
	if s == nil {
		e.push_err(lit.Kind.Token, "invalid_syntax")
		return nil
	}

	ok = e.s.check_generic_quantity(len(s.Decl.Generics), len(s.Generics), lit.Kind.Token)
	if !ok {
		return nil
	}

	// NOTE: Instance already checked if generic quantity passes.

	// TODO: Check pairs.

	return &Data{
		Mutable: true,
		Kind:    t.Kind,
	}
}

func (e *_Eval) eval_expr_kind(kind ast.ExprData) *Data {
	// TODO: Implement other types.
	switch kind.(type) {
	case *ast.LitExpr:
		return e.eval_lit(kind.(*ast.LitExpr))

	case *ast.IdentExpr:
		return e.eval_ident(kind.(*ast.IdentExpr))

	case *ast.UnaryExpr:
		return e.eval_unary(kind.(*ast.UnaryExpr))

	case *ast.VariadicExpr:
		return e.eval_variadic(kind.(*ast.VariadicExpr))

	case *ast.UnsafeExpr:
		return e.eval_unsafe(kind.(*ast.UnsafeExpr))

	case *ast.SliceExpr:
		return e.eval_slice_expr(kind.(*ast.SliceExpr))

	case *ast.IndexingExpr:
		return e.eval_indexing(kind.(*ast.IndexingExpr))

	case *ast.SlicingExpr:
		return e.eval_slicing(kind.(*ast.SlicingExpr))

	case *ast.CastExpr:
		return e.eval_cast(kind.(*ast.CastExpr))

	case *ast.NsSelectionExpr:
		return e.eval_ns_selection(kind.(*ast.NsSelectionExpr))

	case *ast.StructLit:
		return e.eval_struct_lit(kind.(*ast.StructLit))

	default:
		return nil
	}
}

// Returns value data of evaluated expression.
// Returns nil if error occurs.
func (e *_Eval) eval(expr *ast.Expr) *Data {
	data := e.eval_expr_kind(expr.Kind)
	switch {
	case data == nil:
		return nil

	case data.Decl:
		e.push_err(expr.Token, "invalid_expr")
		return nil

	default:
		return data
	}
}
