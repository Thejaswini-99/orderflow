package main

import (
	"context"

	"github.com/Thejaswini-99/orderflow/cmd"
	"github.com/Thejaswini-99/orderflow/proto/paymentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type paymentGRPCServer struct {
	paymentpb.UnimplementedPaymentServiceServer
}

func (s *paymentGRPCServer) CreatePayment(ctx context.Context, req *paymentpb.CreatePaymentRequest) (res *paymentpb.CreatePaymentResponse, err error) {

	if req.Amount <= 0 {
		return nil, status.Error(codes.InvalidArgument, "Amount Must be >0")
	}
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "Invalid Request")
	}

	mu.Lock()
	defer mu.Unlock()

	pid, ok := idempotencyMap[req.IdempotencyKey]

	if ok {
		existing := paymentMap[pid]
		return &paymentpb.CreatePaymentResponse{
			PaymentId: int64(existing.Id),
			Status:    existing.Status,
		}, nil
	}

	payment := Payment{
		Id:      nextPaymentId,
		OrderId: int(req.OrderId),
		Amount:  int(req.Amount),
		Status:  cmd.StatusPending,
	}

	paymentMap[payment.Id] = payment
	idempotencyMap[req.IdempotencyKey] = payment.Id
	nextPaymentId++
	go fakeProvider(payment.Id)
	return &paymentpb.CreatePaymentResponse{
		PaymentId: int64(payment.Id),
		Status:    payment.Status,
	}, nil
}
