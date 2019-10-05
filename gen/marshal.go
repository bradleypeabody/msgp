package gen

import (
	"fmt"
	"io"
	"strings"

	"github.com/bradleypeabody/msgp/msgp"
)

func marshal(w io.Writer) *marshalGen {
	return &marshalGen{
		p: printer{w: w},
	}
}

type marshalGen struct {
	passes
	p    printer
	fuse []byte
	ctx  *Context
	oe   omitemptyGen
}

func (m *marshalGen) Method() Method { return Marshal }

func (m *marshalGen) Apply(dirs []string) error {
	return nil
}

func (m *marshalGen) Execute(p Elem) error {
	if !m.p.ok() {
		return m.p.err
	}
	p = m.applyall(p)
	if p == nil {
		return nil
	}
	if !IsPrintable(p) {
		return nil
	}

	m.ctx = &Context{}

	m.p.comment("MarshalMsg implements msgp.Marshaler")

	// save the vname before
	// calling methodReceiver so
	// that z.Msgsize() is printed correctly
	c := p.Varname()

	m.p.printf("\nfunc (%s %s) MarshalMsg(b []byte) (o []byte, err error) {", p.Varname(), imutMethodReceiver(p))
	m.p.printf("\no = msgp.Require(b, %s.Msgsize())", c)
	next(m, p)
	m.p.nakedReturn()
	return m.p.err
}

func (m *marshalGen) rawAppend(typ string, argfmt string, arg interface{}) {
	m.p.printf("\no = msgp.Append%s(o, %s)", typ, fmt.Sprintf(argfmt, arg))
}

func (m *marshalGen) fuseHook() {
	if len(m.fuse) > 0 {
		m.rawbytes(m.fuse)
		m.fuse = m.fuse[:0]
	}
}

func (m *marshalGen) Fuse(b []byte) {
	if len(m.fuse) == 0 {
		m.fuse = b
	} else {
		m.fuse = append(m.fuse, b...)
	}
}

func (m *marshalGen) gStruct(s *Struct) {
	if !m.p.ok() {
		return
	}

	if s.AsTuple {
		m.tuple(s)
	} else {
		m.mapstruct(s)
	}
	return
}

func (m *marshalGen) tuple(s *Struct) {
	data := make([]byte, 0, 5)
	data = msgp.AppendArrayHeader(data, uint32(len(s.Fields)))
	m.p.printf("\n// array header, size %d", len(s.Fields))
	m.Fuse(data)
	if len(s.Fields) == 0 {
		m.fuseHook()
	}
	for i := range s.Fields {
		if !m.p.ok() {
			return
		}
		m.ctx.PushString(s.Fields[i].FieldName)
		next(m, s.Fields[i].FieldElem)
		m.ctx.Pop()
	}
}

func (m *marshalGen) mapstruct(s *Struct) {

	var data []byte
	nfields := len(s.Fields)

	omitemptyenc := m.oe.isEncForAnyFields(s.Fields)
	var fieldNVar, fieldEMVar string
	if omitemptyenc {

		fieldNVar = strings.ReplaceAll(s.Varname(), ".", "__") + "FieldN"
		fieldEMVar = strings.ReplaceAll(s.Varname(), ".", "__") + "EmptyMask"

		m.p.printf("\n// omitempty: check for empty values")
		m.p.printf("\n%s := %d", fieldNVar, nfields)
		m.p.printf("\nvar %s %s; _ = %s", fieldEMVar, m.oe.bmaskType(nfields), fieldEMVar)
		for i, sf := range s.Fields {
			if !m.p.ok() {
				return
			}
			if m.oe.isEncField(&s.Fields[i]) {
				m.p.printf("\nif %s {", m.oe.emptyExpr(sf.FieldElem, s.Varname()+"."+sf.FieldName))
				m.p.printf("\n%s--", fieldNVar)
				m.p.printf("\n%s", m.oe.bmaskAssign1(fieldEMVar, i, nfields))
				m.p.printf("\n}")
			}
		}

		m.p.printf("\n// dynamic map header, size %s", fieldNVar)
		m.oe.writeMarshalDynMapHeader(&m.p, fieldNVar, nfields)
		if !m.p.ok() {
			return
		}

		// quick return for the case where the entire thing is empty
		m.p.printf("\nif %s == 0 { return }", fieldNVar)
	} else {

		// non-omitempty version
		data = make([]byte, 0, 64)
		data = msgp.AppendMapHeader(data, uint32(len(s.Fields)))
		m.p.printf("\n// map header, size %d", len(s.Fields))
		m.Fuse(data)
		if len(s.Fields) == 0 {
			m.fuseHook()
		}

	}

	for i := range s.Fields {
		if !m.p.ok() {
			return
		}

		// if field is omitempty/omitemptyenc, wrap with if statement based on the emptymask
		oeField := m.oe.isEncField(&s.Fields[i])
		if oeField {
			m.p.printf("\nif %s == 0 { // if not empty", m.oe.bmaskRead(fieldEMVar, i, nfields))
		}

		data = msgp.AppendString(nil, s.Fields[i].FieldTag)

		m.p.printf("\n// string %q", s.Fields[i].FieldTag)
		m.Fuse(data)

		m.ctx.PushString(s.Fields[i].FieldName)
		next(m, s.Fields[i].FieldElem)
		m.ctx.Pop()

		if oeField {
			m.p.printf("\n}") // close if statement
		}

	}
}

// append raw data
func (m *marshalGen) rawbytes(bts []byte) {
	m.p.print("\no = append(o, ")
	for _, b := range bts {
		m.p.printf("0x%x,", b)
	}
	m.p.print(")")
}

func (m *marshalGen) gMap(s *Map) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	vname := s.Varname()
	m.rawAppend(mapHeader, lenAsUint32, vname)
	m.p.printf("\nfor %s, %s := range %s {", s.Keyidx, s.Validx, vname)
	m.rawAppend(stringTyp, literalFmt, s.Keyidx)
	m.ctx.PushVar(s.Keyidx)
	next(m, s.Value)
	m.ctx.Pop()
	m.p.closeblock()
}

func (m *marshalGen) gSlice(s *Slice) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	vname := s.Varname()
	m.rawAppend(arrayHeader, lenAsUint32, vname)
	m.p.rangeBlock(m.ctx, s.Index, vname, m, s.Els)
}

func (m *marshalGen) gArray(a *Array) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	if be, ok := a.Els.(*BaseElem); ok && be.Value == Byte {
		m.rawAppend("Bytes", "(%s)[:]", a.Varname())
		return
	}

	m.rawAppend(arrayHeader, literalFmt, coerceArraySize(a.Size))
	m.p.rangeBlock(m.ctx, a.Index, a.Varname(), m, a.Els)
}

func (m *marshalGen) gPtr(p *Ptr) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	m.p.printf("\nif %s == nil {\no = msgp.AppendNil(o)\n} else {", p.Varname())
	next(m, p.Value)
	m.p.closeblock()
}

func (m *marshalGen) gBase(b *BaseElem) {
	if !m.p.ok() {
		return
	}
	m.fuseHook()
	vname := b.Varname()

	if b.Convert {
		if b.ShimMode == Cast {
			vname = tobaseConvert(b)
		} else {
			vname = randIdent()
			m.p.printf("\nvar %s %s", vname, b.BaseType())
			m.p.printf("\n%s, err = %s", vname, tobaseConvert(b))
			m.p.wrapErrCheck(m.ctx.ArgsStr())
		}
	}

	var echeck bool
	switch b.Value {
	case IDENT:
		echeck = true
		m.p.printf("\no, err = %s.MarshalMsg(o)", vname)
	case Intf, Ext:
		echeck = true
		m.p.printf("\no, err = msgp.Append%s(o, %s)", b.BaseName(), vname)
	default:
		m.rawAppend(b.BaseName(), literalFmt, vname)
	}

	if echeck {
		m.p.wrapErrCheck(m.ctx.ArgsStr())
	}
}
