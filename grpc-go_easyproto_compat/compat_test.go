package main

import (
	"testing"

	"google.golang.org/protobuf/proto"

	easyorder "github.com/ucpr/workspace2026/grpc-go_easyproto_compat/easyproto"
	"github.com/ucpr/workspace2026/grpc-go_easyproto_compat/gen/orderpb"
	"github.com/ucpr/workspace2026/grpc-go_easyproto_compat/testdata"
)

// TestProtoMarshal_EasyUnmarshal verifies that proto.Marshal output
// can be unmarshaled by easyproto.
func TestProtoMarshal_EasyUnmarshal(t *testing.T) {
	protoOrder := testdata.NewProtoOrder()

	data, err := proto.Marshal(protoOrder)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	var easyOrder easyorder.Order
	if err := easyOrder.UnmarshalProtobuf(data); err != nil {
		t.Fatalf("easyproto UnmarshalProtobuf failed: %v", err)
	}

	assertOrderEqual(t, protoOrder, &easyOrder)
}

// TestEasyMarshal_ProtoUnmarshal verifies that easyproto.Marshal output
// can be unmarshaled by proto.Unmarshal.
func TestEasyMarshal_ProtoUnmarshal(t *testing.T) {
	easyOrder := testdata.NewEasyOrder()

	data := easyOrder.MarshalProtobuf(nil)

	var protoOrder orderpb.Order
	if err := proto.Unmarshal(data, &protoOrder); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	assertOrderEqual(t, &protoOrder, easyOrder)
}

// TestRoundTrip_Proto tests standard protobuf marshal/unmarshal round-trip.
func TestRoundTrip_Proto(t *testing.T) {
	original := testdata.NewProtoOrder()

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatalf("proto.Marshal failed: %v", err)
	}

	var decoded orderpb.Order
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("proto.Unmarshal failed: %v", err)
	}

	if !proto.Equal(original, &decoded) {
		t.Fatal("round-trip proto messages are not equal")
	}
}

// TestRoundTrip_Easy tests easyproto marshal/unmarshal round-trip.
func TestRoundTrip_Easy(t *testing.T) {
	original := testdata.NewEasyOrder()

	data := original.MarshalProtobuf(nil)

	var decoded easyorder.Order
	if err := decoded.UnmarshalProtobuf(data); err != nil {
		t.Fatalf("easyproto UnmarshalProtobuf failed: %v", err)
	}

	assertEasyOrderEqual(t, original, &decoded)
}

// TestEmptyMessage_CrossCompat tests cross-compatibility with empty messages.
func TestEmptyMessage_CrossCompat(t *testing.T) {
	t.Run("proto_to_easy", func(t *testing.T) {
		empty := &orderpb.Order{}
		data, err := proto.Marshal(empty)
		if err != nil {
			t.Fatalf("proto.Marshal failed: %v", err)
		}

		var easyOrder easyorder.Order
		if err := easyOrder.UnmarshalProtobuf(data); err != nil {
			t.Fatalf("easyproto UnmarshalProtobuf failed: %v", err)
		}

		if easyOrder.OrderID != "" || easyOrder.Status != 0 || easyOrder.ShippingAddress != nil ||
			len(easyOrder.Items) != 0 || easyOrder.CreatedAt != 0 || easyOrder.IsPriority || easyOrder.TotalAmount != 0 {
			t.Fatal("expected empty order after unmarshaling empty proto message")
		}
	})

	t.Run("easy_to_proto", func(t *testing.T) {
		empty := &easyorder.Order{}
		data := empty.MarshalProtobuf(nil)

		var protoOrder orderpb.Order
		if err := proto.Unmarshal(data, &protoOrder); err != nil {
			t.Fatalf("proto.Unmarshal failed: %v", err)
		}

		if protoOrder.GetOrderId() != "" || protoOrder.GetStatus() != 0 || protoOrder.GetShippingAddress() != nil ||
			len(protoOrder.GetItems()) != 0 || protoOrder.GetCreatedAt() != 0 || protoOrder.GetIsPriority() || protoOrder.GetTotalAmount() != 0 {
			t.Fatal("expected empty order after unmarshaling empty easyproto message")
		}
	})
}

// TestLargeRepeated_CrossCompat tests cross-compatibility with 100 repeated items.
func TestLargeRepeated_CrossCompat(t *testing.T) {
	const numItems = 100

	t.Run("proto_to_easy", func(t *testing.T) {
		protoOrder := &orderpb.Order{
			OrderId: "ORD-LARGE-001",
			Status:  orderpb.OrderStatus_ORDER_STATUS_SHIPPED,
			Items:   testdata.NewProtoOrderItems(numItems),
		}

		data, err := proto.Marshal(protoOrder)
		if err != nil {
			t.Fatalf("proto.Marshal failed: %v", err)
		}

		var easyOrder easyorder.Order
		if err := easyOrder.UnmarshalProtobuf(data); err != nil {
			t.Fatalf("easyproto UnmarshalProtobuf failed: %v", err)
		}

		if len(easyOrder.Items) != numItems {
			t.Fatalf("expected %d items, got %d", numItems, len(easyOrder.Items))
		}
		assertOrderEqual(t, protoOrder, &easyOrder)
	})

	t.Run("easy_to_proto", func(t *testing.T) {
		easyOrder := &easyorder.Order{
			OrderID: "ORD-LARGE-001",
			Status:  easyorder.OrderStatusShipped,
			Items:   testdata.NewEasyOrderItems(numItems),
		}

		data := easyOrder.MarshalProtobuf(nil)

		var protoOrder orderpb.Order
		if err := proto.Unmarshal(data, &protoOrder); err != nil {
			t.Fatalf("proto.Unmarshal failed: %v", err)
		}

		if len(protoOrder.GetItems()) != numItems {
			t.Fatalf("expected %d items, got %d", numItems, len(protoOrder.GetItems()))
		}
		assertOrderEqual(t, &protoOrder, easyOrder)
	})
}

// assertOrderEqual compares a proto Order and an easyproto Order field by field.
func assertOrderEqual(t *testing.T, pb *orderpb.Order, ep *easyorder.Order) {
	t.Helper()

	if pb.GetOrderId() != ep.OrderID {
		t.Errorf("OrderID mismatch: proto=%q, easy=%q", pb.GetOrderId(), ep.OrderID)
	}
	if int32(pb.GetStatus()) != int32(ep.Status) {
		t.Errorf("Status mismatch: proto=%d, easy=%d", pb.GetStatus(), ep.Status)
	}
	if pb.GetCreatedAt() != ep.CreatedAt {
		t.Errorf("CreatedAt mismatch: proto=%d, easy=%d", pb.GetCreatedAt(), ep.CreatedAt)
	}
	if pb.GetIsPriority() != ep.IsPriority {
		t.Errorf("IsPriority mismatch: proto=%v, easy=%v", pb.GetIsPriority(), ep.IsPriority)
	}
	if pb.GetTotalAmount() != ep.TotalAmount {
		t.Errorf("TotalAmount mismatch: proto=%f, easy=%f", pb.GetTotalAmount(), ep.TotalAmount)
	}

	pbAddr := pb.GetShippingAddress()
	epAddr := ep.ShippingAddress
	if (pbAddr == nil) != (epAddr == nil) {
		t.Fatalf("ShippingAddress nil mismatch: proto=%v, easy=%v", pbAddr == nil, epAddr == nil)
	}
	if pbAddr != nil && epAddr != nil {
		if pbAddr.GetStreet() != epAddr.Street {
			t.Errorf("Address.Street mismatch: proto=%q, easy=%q", pbAddr.GetStreet(), epAddr.Street)
		}
		if pbAddr.GetCity() != epAddr.City {
			t.Errorf("Address.City mismatch: proto=%q, easy=%q", pbAddr.GetCity(), epAddr.City)
		}
		if pbAddr.GetState() != epAddr.State {
			t.Errorf("Address.State mismatch: proto=%q, easy=%q", pbAddr.GetState(), epAddr.State)
		}
		if pbAddr.GetZipCode() != epAddr.ZipCode {
			t.Errorf("Address.ZipCode mismatch: proto=%q, easy=%q", pbAddr.GetZipCode(), epAddr.ZipCode)
		}
		if pbAddr.GetCountry() != epAddr.Country {
			t.Errorf("Address.Country mismatch: proto=%q, easy=%q", pbAddr.GetCountry(), epAddr.Country)
		}
	}

	if len(pb.GetItems()) != len(ep.Items) {
		t.Fatalf("Items length mismatch: proto=%d, easy=%d", len(pb.GetItems()), len(ep.Items))
	}
	for i, pbItem := range pb.GetItems() {
		epItem := &ep.Items[i]
		if pbItem.GetProductId() != epItem.ProductID {
			t.Errorf("Items[%d].ProductID mismatch: proto=%q, easy=%q", i, pbItem.GetProductId(), epItem.ProductID)
		}
		if pbItem.GetProductName() != epItem.ProductName {
			t.Errorf("Items[%d].ProductName mismatch: proto=%q, easy=%q", i, pbItem.GetProductName(), epItem.ProductName)
		}
		if pbItem.GetQuantity() != epItem.Quantity {
			t.Errorf("Items[%d].Quantity mismatch: proto=%d, easy=%d", i, pbItem.GetQuantity(), epItem.Quantity)
		}
		if pbItem.GetUnitPrice() != epItem.UnitPrice {
			t.Errorf("Items[%d].UnitPrice mismatch: proto=%f, easy=%f", i, pbItem.GetUnitPrice(), epItem.UnitPrice)
		}
	}
}

// assertEasyOrderEqual compares two easyproto Orders field by field.
func assertEasyOrderEqual(t *testing.T, a, b *easyorder.Order) {
	t.Helper()

	if a.OrderID != b.OrderID {
		t.Errorf("OrderID mismatch: %q vs %q", a.OrderID, b.OrderID)
	}
	if a.Status != b.Status {
		t.Errorf("Status mismatch: %d vs %d", a.Status, b.Status)
	}
	if a.CreatedAt != b.CreatedAt {
		t.Errorf("CreatedAt mismatch: %d vs %d", a.CreatedAt, b.CreatedAt)
	}
	if a.IsPriority != b.IsPriority {
		t.Errorf("IsPriority mismatch: %v vs %v", a.IsPriority, b.IsPriority)
	}
	if a.TotalAmount != b.TotalAmount {
		t.Errorf("TotalAmount mismatch: %f vs %f", a.TotalAmount, b.TotalAmount)
	}

	if (a.ShippingAddress == nil) != (b.ShippingAddress == nil) {
		t.Fatalf("ShippingAddress nil mismatch")
	}
	if a.ShippingAddress != nil {
		aa, ba := a.ShippingAddress, b.ShippingAddress
		if aa.Street != ba.Street || aa.City != ba.City || aa.State != ba.State ||
			aa.ZipCode != ba.ZipCode || aa.Country != ba.Country {
			t.Errorf("ShippingAddress mismatch")
		}
	}

	if len(a.Items) != len(b.Items) {
		t.Fatalf("Items length mismatch: %d vs %d", len(a.Items), len(b.Items))
	}
	for i := range a.Items {
		ai, bi := &a.Items[i], &b.Items[i]
		if ai.ProductID != bi.ProductID || ai.ProductName != bi.ProductName ||
			ai.Quantity != bi.Quantity || ai.UnitPrice != bi.UnitPrice {
			t.Errorf("Items[%d] mismatch", i)
		}
	}
}
