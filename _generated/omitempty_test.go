package _generated

import (
	"bytes"
	"testing"

	"github.com/bradleypeabody/msgp/msgp"
)

func mustEncodeToJSON(o msgp.Encodable) string {
	var buf bytes.Buffer
	var err error

	en := msgp.NewWriter(&buf)
	err = o.EncodeMsg(en)
	if err != nil {
		panic(err)
	}
	en.Flush()

	var outbuf bytes.Buffer
	_, err = msgp.CopyToJSON(&outbuf, &buf)
	if err != nil {
		panic(err)
	}

	return outbuf.String()
}

func TestOmitEmpty0(t *testing.T) {

	var s string

	var oe0a OmitEmpty0

	s = mustEncodeToJSON(&oe0a)
	if s != `{}` {
		t.Errorf("wrong result: %s", s)
	}

	var oe0b OmitEmpty0
	oe0b.AString = "teststr"
	s = mustEncodeToJSON(&oe0b)
	if s != `{"astring":"teststr"}` {
		t.Errorf("wrong result: %s", s)
	}

}

// TestOmitEmptyHalfFull tests mixed omitempty and not
func TestOmitEmptyHalfFull(t *testing.T) {

	var s string

	var oeA OmitEmptyHalfFull

	s = mustEncodeToJSON(&oeA)
	if s != `{"field01":"","field03":""}` {
		t.Errorf("wrong result: %s", s)
	}

	var oeB OmitEmptyHalfFull
	oeB.Field02 = "val2"
	s = mustEncodeToJSON(&oeB)
	if s != `{"field01":"","field02":"val2","field03":""}` {
		t.Errorf("wrong result: %s", s)
	}

	var oeC OmitEmptyHalfFull
	oeC.Field03 = "val3"
	s = mustEncodeToJSON(&oeC)
	if s != `{"field01":"","field03":"val3"}` {
		t.Errorf("wrong result: %s", s)
	}
}

// TestOmitEmptyLotsOFields tests the case of > 64 fields (triggers the bitmask needing to be an array instead of a single value)
func TestOmitEmptyLotsOFields(t *testing.T) {

	var s string

	var oeLotsA OmitEmptyLotsOFields

	s = mustEncodeToJSON(&oeLotsA)
	if s != `{}` {
		t.Errorf("wrong result: %s", s)
	}

	var oeLotsB OmitEmptyLotsOFields
	oeLotsB.Field04 = "val4"
	s = mustEncodeToJSON(&oeLotsB)
	if s != `{"field04":"val4"}` {
		t.Errorf("wrong result: %s", s)
	}

	var oeLotsC OmitEmptyLotsOFields
	oeLotsC.Field64 = "val64"
	s = mustEncodeToJSON(&oeLotsC)
	if s != `{"field64":"val64"}` {
		t.Errorf("wrong result: %s", s)
	}

}

// TODO:
// - omitemptyenc should only do encode
// - omitemptydec should only do decode
// - marshal/unmarshal code path
// - extensions? time?
