// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proto_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/internal/encoding/wire"
	"google.golang.org/protobuf/internal/flags"
	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"

	orderpb "google.golang.org/protobuf/internal/testprotos/order"
	testpb "google.golang.org/protobuf/internal/testprotos/test"
	test3pb "google.golang.org/protobuf/internal/testprotos/test3"
)

func TestEncode(t *testing.T) {
	for _, test := range testProtos {
		for _, want := range test.decodeTo {
			t.Run(fmt.Sprintf("%s (%T)", test.desc, want), func(t *testing.T) {
				opts := proto.MarshalOptions{
					AllowPartial: test.partial,
				}
				wire, err := opts.Marshal(want)
				if err != nil {
					t.Fatalf("Marshal error: %v\nMessage:\n%v", err, marshalText(want))
				}

				size := proto.Size(want)
				if size != len(wire) {
					t.Errorf("Size and marshal disagree: Size(m)=%v; len(Marshal(m))=%v\nMessage:\n%v", size, len(wire), marshalText(want))
				}

				got := want.ProtoReflect().New().Interface()
				uopts := proto.UnmarshalOptions{
					AllowPartial: test.partial,
				}
				if err := uopts.Unmarshal(wire, got); err != nil {
					t.Errorf("Unmarshal error: %v\nMessage:\n%v", err, marshalText(want))
					return
				}
				if !proto.Equal(got, want) {
					t.Errorf("Unmarshal returned unexpected result; got:\n%v\nwant:\n%v", marshalText(got), marshalText(want))
				}
			})
		}
	}
}

func TestEncodeDeterministic(t *testing.T) {
	for _, test := range testProtos {
		for _, want := range test.decodeTo {
			t.Run(fmt.Sprintf("%s (%T)", test.desc, want), func(t *testing.T) {
				opts := proto.MarshalOptions{
					Deterministic: true,
					AllowPartial:  test.partial,
				}
				wire, err := opts.Marshal(want)
				if err != nil {
					t.Fatalf("Marshal error: %v\nMessage:\n%v", err, marshalText(want))
				}
				wire2, err := opts.Marshal(want)
				if err != nil {
					t.Fatalf("Marshal error: %v\nMessage:\n%v", err, marshalText(want))
				}
				if !bytes.Equal(wire, wire2) {
					t.Fatalf("deterministic marshal returned varying results:\n%v", cmp.Diff(wire, wire2))
				}

				got := want.ProtoReflect().New().Interface()
				uopts := proto.UnmarshalOptions{
					AllowPartial: test.partial,
				}
				if err := uopts.Unmarshal(wire, got); err != nil {
					t.Errorf("Unmarshal error: %v\nMessage:\n%v", err, marshalText(want))
					return
				}
				if !proto.Equal(got, want) {
					t.Errorf("Unmarshal returned unexpected result; got:\n%v\nwant:\n%v", marshalText(got), marshalText(want))
				}
			})
		}
	}
}

func TestEncodeInvalidUTF8(t *testing.T) {
	for _, test := range invalidUTF8TestProtos {
		for _, want := range test.decodeTo {
			t.Run(fmt.Sprintf("%s (%T)", test.desc, want), func(t *testing.T) {
				_, err := proto.Marshal(want)
				if err == nil {
					t.Errorf("Marshal did not return expected error for invalid UTF8: %v\nMessage:\n%v", err, marshalText(want))
				}
			})
		}
	}
}

func TestEncodeNoEnforceUTF8(t *testing.T) {
	for _, test := range noEnforceUTF8TestProtos {
		for _, want := range test.decodeTo {
			t.Run(fmt.Sprintf("%s (%T)", test.desc, want), func(t *testing.T) {
				_, err := proto.Marshal(want)
				switch {
				case flags.ProtoLegacy && err != nil:
					t.Errorf("Marshal returned unexpected error: %v\nMessage:\n%v", err, marshalText(want))
				case !flags.ProtoLegacy && err == nil:
					t.Errorf("Marshal did not return expected error for invalid UTF8: %v\nMessage:\n%v", err, marshalText(want))
				}
			})
		}
	}
}

func TestEncodeRequiredFieldChecks(t *testing.T) {
	for _, test := range testProtos {
		if !test.partial {
			continue
		}
		for _, m := range test.decodeTo {
			t.Run(fmt.Sprintf("%s (%T)", test.desc, m), func(t *testing.T) {
				_, err := proto.Marshal(m)
				if err == nil {
					t.Fatalf("Marshal succeeded (want error)\nMessage:\n%v", marshalText(m))
				}
			})
		}
	}
}

func TestEncodeAppend(t *testing.T) {
	want := []byte("prefix")
	got := append([]byte(nil), want...)
	got, err := proto.MarshalOptions{}.MarshalAppend(got, &test3pb.TestAllTypes{
		OptionalString: "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, want) {
		t.Fatalf("MarshalAppend modified prefix: got %v, want prefix %v", got, want)
	}
}

func TestEncodeOneofNilWrapper(t *testing.T) {
	m := &testpb.TestAllTypes{OneofField: (*testpb.TestAllTypes_OneofUint32)(nil)}
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 0 {
		t.Errorf("Marshal return non-empty, want empty")
	}
}

func TestMarshalAppendAllocations(t *testing.T) {
	m := &test3pb.TestAllTypes{OptionalInt32: 1}
	size := proto.Size(m)
	const count = 1000
	b := make([]byte, size)
	// AllocsPerRun returns an integral value.
	marshalAllocs := testing.AllocsPerRun(count, func() {
		_, err := proto.MarshalOptions{}.MarshalAppend(b[:0], m)
		if err != nil {
			t.Fatal(err)
		}
	})
	b = nil
	marshalAppendAllocs := testing.AllocsPerRun(count, func() {
		var err error
		b, err = proto.MarshalOptions{}.MarshalAppend(b, m)
		if err != nil {
			t.Fatal(err)
		}
	})
	if marshalAllocs != marshalAppendAllocs {
		t.Errorf("%v allocs/op when writing to a preallocated buffer", marshalAllocs)
		t.Errorf("%v allocs/op when repeatedly appending to a slice", marshalAppendAllocs)
		t.Errorf("expect amortized allocs/op to be identical")
	}
}

func TestEncodeOrder(t *testing.T) {
	// We make no guarantees about the stability of wire marshal output.
	// The order in which fields are marshaled may change over time.
	// If deterministic marshaling is not enabled, it may change over
	// successive calls to proto.Marshal in the same binary.
	//
	// Unfortunately, many users have come to rely on the specific current
	// wire marshal output. Perhaps someday we will choose to deliberately
	// change the marshal output; until that day comes, this test verifies
	// that we don't unintentionally change it.
	m := &orderpb.Message{
		Field_1:  proto.String("one"),
		Field_2:  proto.String("two"),
		Field_20: proto.String("twenty"),
		Oneof_1:  &orderpb.Message_Field_10{"ten"},
	}
	proto.SetExtension(m, orderpb.E_Field_30, "thirty")
	proto.SetExtension(m, orderpb.E_Field_31, "thirty-one")
	proto.SetExtension(m, orderpb.E_Field_32, "thirty-two")
	want := []pref.FieldNumber{
		30, 31, 32, // extensions first, in number order
		1, 2, 20, // non-extension, non-oneof in number order
		10, // oneofs last, undefined order
	}

	// Test with deterministic serialization, since fields are not sorted without
	// it when -tags=protoreflect.
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var got []pref.FieldNumber
	for len(b) > 0 {
		num, _, n := wire.ConsumeField(b)
		if n < 0 {
			t.Fatal(wire.ParseError(n))
		}
		b = b[n:]
		got = append(got, num)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected field marshal order:\ngot:  %v\nwant: %v\nmessage:\n%v", got, want, m)
	}
}
