package controller

import (
	"context"

	"github.com/teakingwang/grpcgwmicro/api/order"
	"github.com/teakingwang/grpcgwmicro/internal/order/service"
)

type OrderController struct {
	order.UnimplementedOrderServiceServer
	svc service.OrderService
}

func NewOrderController(svc service.OrderService) *OrderController {
	return &OrderController{svc: svc}
}

func (uc *OrderController) GetOrder(ctx context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	u, err := uc.svc.GetOrder(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if u == nil {
		return &order.GetOrderResponse{}, nil
	}

	return &order.GetOrderResponse{
		OrderID:  u.OrderID,
		OrderSN:  u.OrderSN,
		UserID:   u.UserID,
		Username: u.Username,
	}, nil
}
