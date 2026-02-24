package easyproto

import (
	"fmt"

	"github.com/VictoriaMetrics/easyproto"
)

// OrderStatus corresponds to orderpb.OrderStatus enum.
type OrderStatus int32

const (
	OrderStatusUnspecified OrderStatus = 0
	OrderStatusPending     OrderStatus = 1
	OrderStatusConfirmed   OrderStatus = 2
	OrderStatusShipped     OrderStatus = 3
	OrderStatusDelivered   OrderStatus = 4
	OrderStatusCancelled   OrderStatus = 5
)

// Address corresponds to orderpb.Address.
type Address struct {
	Street  string
	City    string
	State   string
	ZipCode string
	Country string
}

// OrderItem corresponds to orderpb.OrderItem.
type OrderItem struct {
	ProductID   string
	ProductName string
	Quantity    int32
	UnitPrice   float64
}

// Order corresponds to orderpb.Order.
type Order struct {
	OrderID         string
	Status          OrderStatus
	ShippingAddress *Address
	Items           []OrderItem
	CreatedAt       int64
	IsPriority      bool
	TotalAmount     float64
}

var mp easyproto.MarshalerPool

// MarshalProtobuf marshals the Order to protobuf wire format.
func (o *Order) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	o.marshalProtobuf(m.MessageMarshaler())
	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

func (o *Order) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	// Field 1: order_id (string)
	if o.OrderID != "" {
		mm.AppendString(1, o.OrderID)
	}
	// Field 2: status (enum, varint)
	if o.Status != 0 {
		mm.AppendInt32(2, int32(o.Status))
	}
	// Field 3: shipping_address (nested message)
	if o.ShippingAddress != nil {
		o.ShippingAddress.marshalProtobuf(mm.AppendMessage(3))
	}
	// Field 4: items (repeated message)
	for i := range o.Items {
		o.Items[i].marshalProtobuf(mm.AppendMessage(4))
	}
	// Field 5: created_at (int64)
	if o.CreatedAt != 0 {
		mm.AppendInt64(5, o.CreatedAt)
	}
	// Field 6: is_priority (bool)
	if o.IsPriority {
		mm.AppendBool(6, o.IsPriority)
	}
	// Field 7: total_amount (double)
	if o.TotalAmount != 0 {
		mm.AppendDouble(7, o.TotalAmount)
	}
}

// UnmarshalProtobuf unmarshals the Order from protobuf wire format.
func (o *Order) UnmarshalProtobuf(src []byte) error {
	o.OrderID = ""
	o.Status = 0
	o.ShippingAddress = nil
	o.Items = o.Items[:0]
	o.CreatedAt = 0
	o.IsPriority = false
	o.TotalAmount = 0

	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Order: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Order.order_id")
			}
			o.OrderID = s
		case 2:
			v, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read Order.status")
			}
			o.Status = OrderStatus(v)
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Order.shipping_address")
			}
			o.ShippingAddress = &Address{}
			if err := o.ShippingAddress.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Order.shipping_address: %w", err)
			}
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Order.items")
			}
			o.Items = append(o.Items, OrderItem{})
			item := &o.Items[len(o.Items)-1]
			if err := item.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Order.items: %w", err)
			}
		case 5:
			v, ok := fc.Int64()
			if !ok {
				return fmt.Errorf("cannot read Order.created_at")
			}
			o.CreatedAt = v
		case 6:
			v, ok := fc.Bool()
			if !ok {
				return fmt.Errorf("cannot read Order.is_priority")
			}
			o.IsPriority = v
		case 7:
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read Order.total_amount")
			}
			o.TotalAmount = v
		}
	}
	return nil
}

func (a *Address) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if a.Street != "" {
		mm.AppendString(1, a.Street)
	}
	if a.City != "" {
		mm.AppendString(2, a.City)
	}
	if a.State != "" {
		mm.AppendString(3, a.State)
	}
	if a.ZipCode != "" {
		mm.AppendString(4, a.ZipCode)
	}
	if a.Country != "" {
		mm.AppendString(5, a.Country)
	}
}

// UnmarshalProtobuf unmarshals the Address from protobuf wire format.
func (a *Address) UnmarshalProtobuf(src []byte) error {
	a.Street = ""
	a.City = ""
	a.State = ""
	a.ZipCode = ""
	a.Country = ""

	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Address: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address.street")
			}
			a.Street = s
		case 2:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address.city")
			}
			a.City = s
		case 3:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address.state")
			}
			a.State = s
		case 4:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address.zip_code")
			}
			a.ZipCode = s
		case 5:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address.country")
			}
			a.Country = s
		}
	}
	return nil
}

func (item *OrderItem) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if item.ProductID != "" {
		mm.AppendString(1, item.ProductID)
	}
	if item.ProductName != "" {
		mm.AppendString(2, item.ProductName)
	}
	if item.Quantity != 0 {
		mm.AppendInt32(3, item.Quantity)
	}
	if item.UnitPrice != 0 {
		mm.AppendDouble(4, item.UnitPrice)
	}
}

// UnmarshalProtobuf unmarshals the OrderItem from protobuf wire format.
func (item *OrderItem) UnmarshalProtobuf(src []byte) error {
	item.ProductID = ""
	item.ProductName = ""
	item.Quantity = 0
	item.UnitPrice = 0

	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in OrderItem: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read OrderItem.product_id")
			}
			item.ProductID = s
		case 2:
			s, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read OrderItem.product_name")
			}
			item.ProductName = s
		case 3:
			v, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read OrderItem.quantity")
			}
			item.Quantity = v
		case 4:
			v, ok := fc.Double()
			if !ok {
				return fmt.Errorf("cannot read OrderItem.unit_price")
			}
			item.UnitPrice = v
		}
	}
	return nil
}
