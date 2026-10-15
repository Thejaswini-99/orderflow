package main

import (
	"context"
	"sync"

	"github.com/Thejaswini-99/orderflow/proto/inventorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type inventoryGRPCServer struct {
	inventorypb.UnimplementedInventoryServiceServer
}

var (
	mu sync.Mutex
)

func (s *inventoryGRPCServer) ReserveStock(ctx context.Context, req *inventorypb.ReserveStockRequest) (res *inventorypb.ReserveStockResponse, err error) {
	mu.Lock()
	defer mu.Unlock()
	if req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "Please Enter the Valid Quantity")

	}

	for i := range products {
		if products[i].Id == int(req.ProductId) {
			if products[i].Stock >= int(req.Quantity) {
				products[i].Stock -= int(req.Quantity)
				return &inventorypb.ReserveStockResponse{
					Reserved: true,
					Message:  "Stock Reserved",
				}, nil
			} else {
				return &inventorypb.ReserveStockResponse{
					Reserved: false,
					Message:  "insufficient stock",
				}, nil
			}
		}
	}
	return &inventorypb.ReserveStockResponse{
		Reserved: false,
		Message:  "Product Not Found",
	}, nil

}
