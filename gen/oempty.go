package gen

import (
	"bytes"
	"fmt"
	"math"
)

/*
NOTES:

Primitive types and named structs we can support.

Zero values for primitives are easy.  For a named struct it's Name{}.

Need a way to dynamically generate the map header without requiring msgp import.
(Just doing it "manually" ended up rather simple.)

If omitempty is not supported on a field just silently revert to non-omitempty behavior
(avoids unnecessary breakage, follows behavior of encoding/json and other libs).

Use a bitmask to record which fields are empty. An array of masks are used if there are more fields
than fit in a single mask value (unit64). Aside from using a bit less memory, it increases the odds
that this mask will end up in a register and should perform faster than e.g. using []bool,
which requires a separate byte for each field.

QUESTIONS:
- Should I adjust MsgSize() to look for empties?
- Should FieldTagParts be there or should I just parse OmitEmptyEnc bool as a flag directly on StructField when that is parsed?
- Should oempty.go have emptyExpr or is there a more appropriate place (closer to the type information itself, like in elem.go) to put this?

*/

// omitemptyGen houses the omitempty code generation functionality.
// An instance is included on encodeGen, decodeGen, marshalGen, unmarshalGen
type omitemptyGen struct {
}

// isEncForAnyFields returns true if isEncField returns true for any of these struct fields
func (o *omitemptyGen) isEncForAnyFields(sfs []StructField) bool {
	for i := range sfs {
		if o.isEncField(&sfs[i]) {
			return true
		}
	}
	return false
}

// isEncField returns true if this field is tagged "omitempty"
// or "omitemptyenc" and is of a supported type
func (o *omitemptyGen) isEncField(sf *StructField) bool {

	if len(sf.FieldTagParts) < 2 {
		return false
	}

	for _, p := range sf.FieldTagParts[1:] {
		if p == "omitempty" || p == "omitemptyenc" {
			goto tagok
		}
	}
	return false

tagok:

	return o.isSupportedElem(sf.FieldElem)

}

// isDecForAnyFields returns true if isDecField returns true for any of these struct fields
func (o *omitemptyGen) isDecForAnyFields(sfs []StructField) bool {
	for i := range sfs {
		if o.isDecField(&sfs[i]) {
			return true
		}
	}
	return false
}

// isDecField returns true if this field is tagged "omitempty"
// or "omitemptyDec" and is of a supported type
func (o *omitemptyGen) isDecField(sf *StructField) bool {

	if len(sf.FieldTagParts) < 2 {
		return false
	}

	for _, p := range sf.FieldTagParts[1:] {
		if p == "omitempty" || p == "omitemptydec" {
			goto tagok
		}
	}
	return false

tagok:

	return o.isSupportedElem(sf.FieldElem)

}

// isSupportedElem returns true if this is a type that we can
// code generate omitempty functionality for
func (o *omitemptyGen) isSupportedElem(elem Elem) bool {

	// log.Printf("sf = %#v", sf)

	if el, ok := elem.(*BaseElem); ok {

		switch el.Value {
		case Bytes,
			String,
			Float32,
			Float64,
			Complex64,
			Complex128,
			Uint,
			Uint8,
			Uint16,
			Uint32,
			Uint64,
			Byte,
			Int,
			Int8,
			Int16,
			Int32,
			Int64,
			Bool:
			// Time:
			return true
		}

		return false

		// _ = el
		// p := el.Value
		// log.Printf("TypeName() = %q; p = %#v", el.TypeName(), p)
		// return false
	}

	// sf.FieldElem.hidden
	return false

}

// emptyAssign returns the Go assignment of any empty value
func (o *omitemptyGen) emptyAssign(elem Elem, varname string) string {

	if el, ok := elem.(*BaseElem); ok {

		switch el.Value {
		case Bytes:
			return fmt.Sprintf("%s = nil", varname)
		case String:
			return fmt.Sprintf("%s = \"\"", varname)
		case Complex64, Complex128:
			return fmt.Sprintf("%s = complex(0,0)", varname)
		case Float32,
			Float64,
			Uint,
			Uint8,
			Uint16,
			Uint32,
			Uint64,
			Byte,
			Int,
			Int8,
			Int16,
			Int32,
			Int64:
			return fmt.Sprintf("%s = 0", varname)
		case Bool:
			return fmt.Sprintf("%s = false", varname)

			// case Time:
			// 	return "time.Time{}"
		}

	}

	panic(fmt.Errorf("unsupported Elem: %#v", elem))

}

// emptyExpr returns the Go language expression to compare to empty/zero value for this Elem.
// Passing an unsupported elem will panic (check with isSupportedElem first).
func (o *omitemptyGen) emptyExpr(elem Elem, varname string) string {

	// NOTES:
	// bools are false, named types are Name(false)
	// integer and float types are all "0", named types are Name(0)
	// strings are "", named types are Name("")
	// structs are Struct{} (must be named, unnamed struct types are not supported)
	// pointers, slices and maps are nil, or Name(nil)

	if el, ok := elem.(*BaseElem); ok {

		switch el.Value {
		case Bytes:
			return fmt.Sprintf("len(%s) == 0", varname)
		case String:
			return fmt.Sprintf("len(%s) == 0", varname)
		case Complex64, Complex128:
			return fmt.Sprintf("%s == complex(0,0)", varname)
		case Float32,
			Float64,
			Uint,
			Uint8,
			Uint16,
			Uint32,
			Uint64,
			Byte,
			Int,
			Int8,
			Int16,
			Int32,
			Int64:
			return fmt.Sprintf("%s == 0", varname)
		case Bool:
			return fmt.Sprintf("!%s", varname)

			// case Time:
			// 	return "time.Time{}"
		}

	}

	panic(fmt.Errorf("unsupported Elem: %#v", elem))
}

// bmaskType returns the type to use based on the number of fields required.
// For smaller values it will return the corresponding uintX type,
// if it does not fit int a uint64 then an array of uint64 of the
// required length is returned.
func (o *omitemptyGen) bmaskType(ln int) string {
	if ln <= 8 {
		return "uint8"
	}
	if ln <= 16 {
		return "uint16"
	}
	if ln <= 32 {
		return "uint32"
	}
	if ln <= 64 {
		return "uint64"
	}
	return fmt.Sprintf("[%d]uint64", (ln>>6)+1) // (ln/64)+1
}

// bmaskRead returns a Go expression that reads the bit for the given offset,
// with the other bits masked out.  The offset should be zero-based.
// ln is the total supported length as
// provided to bmaskType.  The resulting expression can be compared to 0.
func (o *omitemptyGen) bmaskRead(varname string, offset, ln int) string {

	var buf bytes.Buffer
	buf.Grow(len(varname) + 16)
	buf.WriteByte('(')
	buf.WriteString(varname)
	if ln > 64 {
		fmt.Fprintf(&buf, "[%d]", (offset >> 6))
	}
	buf.WriteByte('&')
	fmt.Fprintf(&buf, "0x%X", (uint64(1) << (uint64(offset) & 0x3F)))
	buf.WriteByte(')')

	return buf.String()
}

// bmaskAssign1 returns the Go statement which sets the bit at position offset
// to 1.  Rules are otherwise the same as bmaskRead and bmaskType
func (o *omitemptyGen) bmaskAssign1(varname string, offset, ln int) string {

	var buf bytes.Buffer
	buf.Grow(len(varname) + 16)
	buf.WriteString(varname)
	if ln > 64 {
		fmt.Fprintf(&buf, "[%d]", (offset >> 6))
	}
	fmt.Fprintf(&buf, " |= 0x%X", (uint64(1) << (uint64(offset) & 0x3F)))

	return buf.String()
}

// writeMarshalDynMapHeader is like writeEncodeDynMapHeader but for MarshalMsg
func (o *omitemptyGen) writeMarshalDynMapHeader(p *printer, sizeVarname string, maxSize int) {

	if maxSize <= 15 {
		p.printf("\no = append(o, 0x80 | (uint8(%s)&0x0F))", sizeVarname)
	} else if maxSize <= math.MaxUint16 {
		p.printf("\nswitch {")
		p.printf("\ncase %s <= 15:", sizeVarname)
		p.printf("\no = append(o, 0x80 | (uint8(%s)&0x0F))", sizeVarname)
		p.printf("\ndefault:")
		p.printf("\no = append(o, 0xde, uint8(%s>>8), uint8(%s))", sizeVarname, sizeVarname)
		p.printf("\n}")
	} else {
		p.printf("\nswitch {")
		p.printf("\ncase %s <= 15:", sizeVarname)
		p.printf("\no = append(o, 0x80 | (uint8(%s)&0x0F))", sizeVarname)
		p.printf("\ncase %s <= %d:", sizeVarname, math.MaxUint16)
		p.printf("\no = append(o, 0xde, uint8(%s>>8), uint8(%s))", sizeVarname, sizeVarname)
		p.printf("\ndefault:")
		p.printf("\no = append(o, 0xdf, uint8(%s>>24), uint8(%s>>16), uint8(%s>>8) uint8(%s))", sizeVarname, sizeVarname, sizeVarname, sizeVarname)
		p.printf("\n}")
	}

}

// writeDynMapHeader will emit the necessary code which outputs a map
// header to an Encoder (en) using the length in the specified variable (as opposed to
// the static length approach otherwise used).  maxSize is used to avoid emitting code for
// cases of impossible lengths (since we know how many struct fields there are).
func (o *omitemptyGen) writeEncodeDynMapHeader(p *printer, sizeVarname string, maxSize int) {

	if maxSize <= 15 {
		p.printf("\nerr = en.Append(0x80 | (uint8(%s)&0x0F))", sizeVarname)
	} else if maxSize <= math.MaxUint16 {
		p.printf("\nswitch {")
		p.printf("\ncase %s <= 15:", sizeVarname)
		p.printf("\nerr = en.Append(0x80 | (uint8(%s)&0x0F))", sizeVarname)
		p.printf("\ndefault:")
		p.printf("\nerr = en.Append(0xde, uint8(%s>>8), uint8(%s))", sizeVarname, sizeVarname)
		p.printf("\n}")
	} else {
		p.printf("\nswitch {")
		p.printf("\ncase %s <= 15:", sizeVarname)
		p.printf("\nerr = en.Append(0x80 | (uint8(%s)&0x0F))", sizeVarname)
		p.printf("\ncase %s <= %d:", sizeVarname, math.MaxUint16)
		p.printf("\nerr = en.Append(0xde, uint8(%s>>8), uint8(%s))", sizeVarname, sizeVarname)
		p.printf("\ndefault:")
		p.printf("\nerr = en.Append(0xdf, uint8(%s>>24), uint8(%s>>16), uint8(%s>>8) uint8(%s))", sizeVarname, sizeVarname, sizeVarname, sizeVarname)
		p.printf("\n}")
	}

	p.print("\nif err != nil { return }")

}
