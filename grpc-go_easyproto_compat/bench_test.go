package main

import (
	"testing"

	"google.golang.org/protobuf/proto"

	easyorder "github.com/ucpr/workspace2026/grpc-go_easyproto_compat/easyproto"
	"github.com/ucpr/workspace2026/grpc-go_easyproto_compat/gen/orderpb"
	"github.com/ucpr/workspace2026/grpc-go_easyproto_compat/testdata"
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
