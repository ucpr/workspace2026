package testdata

import (
	"fmt"

	easyorder "github.com/ucpr/workspace2026/grpc-go_easyproto_compat/easyproto"
	"github.com/ucpr/workspace2026/grpc-go_easyproto_compat/gen/orderpb"
)

// NewProtoOrder returns a sample Order using the standard protobuf type.
func NewProtoOrder() *orderpb.Order {
	return &orderpb.Order{
		OrderId: "ORD-20260101-001",
		Status:  orderpb.OrderStatus_ORDER_STATUS_CONFIRMED,
		ShippingAddress: &orderpb.Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
			Country: "US",
		},
		Items: []*orderpb.OrderItem{
			{
				ProductId:   "PROD-001",
				ProductName: "Mechanical Keyboard",
				Quantity:    2,
				UnitPrice:   149.99,
			},
			{
				ProductId:   "PROD-002",
				ProductName: "USB-C Cable",
				Quantity:    5,
				UnitPrice:   12.50,
			},
		},
		CreatedAt:   1735689600,
		IsPriority:  true,
		TotalAmount: 362.48,
	}
}

// NewEasyOrder returns a sample Order using the easyproto type.
// The data is identical to NewProtoOrder.
func NewEasyOrder() *easyorder.Order {
	return &easyorder.Order{
		OrderID: "ORD-20260101-001",
		Status:  easyorder.OrderStatusConfirmed,
		ShippingAddress: &easyorder.Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
			Country: "US",
		},
		Items: []easyorder.OrderItem{
			{
				ProductID:   "PROD-001",
				ProductName: "Mechanical Keyboard",
				Quantity:    2,
				UnitPrice:   149.99,
			},
			{
				ProductID:   "PROD-002",
				ProductName: "USB-C Cable",
				Quantity:    5,
				UnitPrice:   12.50,
			},
		},
		CreatedAt:   1735689600,
		IsPriority:  true,
		TotalAmount: 362.48,
	}
}

// NewProtoOrderItems returns n OrderItems for large repeated field tests.
func NewProtoOrderItems(n int) []*orderpb.OrderItem {
	items := make([]*orderpb.OrderItem, n)
	for i := range items {
		items[i] = &orderpb.OrderItem{
			ProductId:   fmt.Sprintf("PROD-%04d", i),
			ProductName: fmt.Sprintf("Product %d", i),
			Quantity:    int32(i + 1),
			UnitPrice:   float64(i)*10.0 + 0.99,
		}
	}
	return items
}

// NewEasyOrderItems returns n OrderItems for large repeated field tests.
func NewEasyOrderItems(n int) []easyorder.OrderItem {
	items := make([]easyorder.OrderItem, n)
	for i := range items {
		items[i] = easyorder.OrderItem{
			ProductID:   fmt.Sprintf("PROD-%04d", i),
			ProductName: fmt.Sprintf("Product %d", i),
			Quantity:    int32(i + 1),
			UnitPrice:   float64(i)*10.0 + 0.99,
		}
	}
	return items
}
