package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Thejaswini-99/orderflow/cmd"
	"github.com/Thejaswini-99/orderflow/proto/inventorypb"
	"github.com/Thejaswini-99/orderflow/proto/paymentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Product struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
	Stock int    `json:"stock"`
}

type OrderItems struct {
	ProductId int `json:"product_id"`
	Quantity  int `json:"quantity"`
}
type createOrderRequest struct {
	CustomerId int          `json:"customer_id"`
	Items      []OrderItems `json:"items"`
}
type Order struct {
	Id         int          `json:"order_id"`
	CustomerId int          `json:"customer_id"`
	Items      []OrderItems `json:"items"`
	Total      int          `json:"total"`
	Status     string       `json:"status"`
}

var (
	mu              sync.Mutex
	orders          = map[int]Order{}
	nextOrderId     = 1
	paymentClient   paymentpb.PaymentServiceClient
	inventoryClient inventorypb.InventoryServiceClient
)

var products = []Product{
	{1, "Laptop", 50000, 20},
	{2, "Televison", 25000, 30},
	{3, "Mouse", 3500, 20},
}

func getOrder(w http.ResponseWriter, r *http.Request) {

	strid := r.PathValue("id")

	id, err := strconv.Atoi(strid)
	if err != nil {
		http.Error(w, "Invalid Id", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	order, ok := orders[id]
	if !ok {
		http.Error(w, "Order Not Found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)

}
func createOrder(w http.ResponseWriter, r *http.Request) {

	var req createOrderRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		http.Error(w, "order must have at least one item", http.StatusBadRequest)
		return
	}

	for _, item := range req.Items {
		resp := reserveStock(item.ProductId, item.Quantity)
		if resp == nil {
			http.Error(w, "Inventory Service Down", http.StatusInternalServerError)
			return
		}
		if !resp.Reserved {
			http.Error(w, "Out Of Stock", http.StatusBadRequest)
			return
		}

	}

	// 1. Order create (lock lopala)
	order, code, msg := placeOrder(req)
	if code != 0 {
		http.Error(w, msg, code)
		return
	}
	// 2. Payment-service ki gRPC call (lock BAYATA) ← ippudu dial chesthunnam 📞
	resp, err := startPayment(order.Id, order.Total)
	if err != nil {
		log.Println("payment call failed:", err)
		order.Status = "PAYMENT_FAILED"
	} else {
		log.Println("payment started: id =", resp.PaymentId, "status =", resp.Status)
		order.Status = "PAYMENT_PENDING"
	}

	if err != nil {
		log.Println("Payment Fialed")
		order.Status = "PAYMENT_FAILED"
	} else {
		log.Println("Payment is in penfing state")
		order.Status = "PAYMENT_PENDING"
	}
	// 3. Updated status save
	mu.Lock()
	orders[order.Id] = order
	mu.Unlock()

	// 4. Response: header → 201 → body
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func main() {

	payconn, err := grpc.NewClient("127.0.0.1:9092", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create gRPC client: %v", err)
	}
	defer payconn.Close()

	paymentClient = paymentpb.NewPaymentServiceClient(payconn)

	invconn, err := grpc.NewClient("127.0.0.1:9093", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create gRPC client: %v", err)
	}

	defer invconn.Close()

	inventoryClient = inventorypb.NewInventoryServiceClient(invconn)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /order/{id}", getOrder)
	mux.HandleFunc("POST /orders", createOrder)

	if err := http.ListenAndServe(":8081", mux); err != nil {
		panic(err)
	}

}

func startPayment(orderID, amount int) (*paymentpb.CreatePaymentResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return paymentClient.CreatePayment(ctx, &paymentpb.CreatePaymentRequest{
		OrderId:        int64(orderID),
		Amount:         int64(amount),
		IdempotencyKey: fmt.Sprintf("order-%d", orderID),
	})
}

func reserveStock(Id, Stock int) *inventorypb.ReserveStockResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	response, err := inventoryClient.ReserveStock(ctx, &inventorypb.ReserveStockRequest{
		ProductId: int64(Id),
		Quantity:  int64(Stock),
	})
	if err != nil {
		return nil
	}
	return response
}

func placeOrder(req createOrderRequest) (Order, int, string) {
	mu.Lock()
	defer mu.Unlock()

	total := 0
	for _, item := range req.Items {
		idx, ok := findProduct(item.ProductId)
		if !ok {
			return Order{}, http.StatusBadRequest, "product not found"
		}
		total += products[idx].Price * item.Quantity
	}

	order := Order{
		Id:         nextOrderId,
		CustomerId: req.CustomerId,
		Items:      req.Items,
		Total:      total,
		Status:     cmd.StatusCreated,
	}
	orders[order.Id] = order
	nextOrderId++
	return order, 0, ""
}
