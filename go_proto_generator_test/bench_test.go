package main

import (
	"testing"

	"google.golang.org/protobuf/proto"

	easyorder "github.com/ucpr/workspace2026/go_proto_generator_test/easyproto"
	"github.com/ucpr/workspace2026/go_proto_generator_test/gen/orderpb"
	"github.com/ucpr/workspace2026/go_proto_generator_test/testdata"
)

func BenchmarkMarshal_Proto(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := proto.Marshal(order)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

func BenchmarkMarshal_EasyProto(b *testing.B) {
	order := testdata.NewEasyOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data := order.MarshalProtobuf(nil)
		_ = data
	}
}

func BenchmarkUnmarshal_Proto(b *testing.B) {
	order := testdata.NewProtoOrder()
	data, err := proto.Marshal(order)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out orderpb.Order
		if err := proto.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal_EasyProto(b *testing.B) {
	order := testdata.NewEasyOrder()
	data := order.MarshalProtobuf(nil)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out easyorder.Order
		if err := out.UnmarshalProtobuf(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTrip_Proto(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := proto.Marshal(order)
		if err != nil {
			b.Fatal(err)
		}
		var out orderpb.Order
		if err := proto.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTrip_EasyProto(b *testing.B) {
	order := testdata.NewEasyOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data := order.MarshalProtobuf(nil)
		var out easyorder.Order
		if err := out.UnmarshalProtobuf(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCross_ProtoMarshal_EasyUnmarshal(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := proto.Marshal(order)
		if err != nil {
			b.Fatal(err)
		}
		var out easyorder.Order
		if err := out.UnmarshalProtobuf(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCross_EasyMarshal_ProtoUnmarshal(b *testing.B) {
	order := testdata.NewEasyOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data := order.MarshalProtobuf(nil)
		var out orderpb.Order
		if err := proto.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

// --- vtprotobuf benchmarks ---

func BenchmarkMarshal_VTProto(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := order.MarshalVT()
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

func BenchmarkUnmarshal_VTProto(b *testing.B) {
	order := testdata.NewProtoOrder()
	data, err := order.MarshalVT()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var out orderpb.Order
		if err := out.UnmarshalVT(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTrip_VTProto(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := order.MarshalVT()
		if err != nil {
			b.Fatal(err)
		}
		var out orderpb.Order
		if err := out.UnmarshalVT(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCross_VTMarshal_ProtoUnmarshal(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := order.MarshalVT()
		if err != nil {
			b.Fatal(err)
		}
		var out orderpb.Order
		if err := proto.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCross_ProtoMarshal_VTUnmarshal(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := proto.Marshal(order)
		if err != nil {
			b.Fatal(err)
		}
		var out orderpb.Order
		if err := out.UnmarshalVT(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCross_VTMarshal_EasyUnmarshal(b *testing.B) {
	order := testdata.NewProtoOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data, err := order.MarshalVT()
		if err != nil {
			b.Fatal(err)
		}
		var out easyorder.Order
		if err := out.UnmarshalProtobuf(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCross_EasyMarshal_VTUnmarshal(b *testing.B) {
	order := testdata.NewEasyOrder()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		data := order.MarshalProtobuf(nil)
		var out orderpb.Order
		if err := out.UnmarshalVT(data); err != nil {
			b.Fatal(err)
		}
	}
}
