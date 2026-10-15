package main

import (
	"crypto/hmac"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/Thejaswini-99/orderflow/cmd"
	"github.com/Thejaswini-99/orderflow/proto/paymentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type CreatePaymentRequest struct {
	OrderId int `json:"order_id"`
	Amount  int `json:"amount"`
}

type Payment struct {
	Id      int    `json:"payment_id"`
	OrderId int    `json:"order_id"`
	Amount  int    `json:"amount"`
	Status  string `json:"status"`
}

var (
	mu             sync.Mutex
	paymentMap     = map[int]Payment{}
	idempotencyMap = map[string]int{}
	nextPaymentId  = 1
)

func createPayment(w http.ResponseWriter, r *http.Request) {

	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		http.Error(w, "Empty Idempotency Key", http.StatusBadRequest)
		return
	}
	var req CreatePaymentRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Amount <= 0 {
		http.Error(w, "Please Enter Valid amount", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	paymentId, ok := idempotencyMap[key]
	if ok {
		idemPayment := paymentMap[paymentId]
		if idemPayment.OrderId != req.OrderId || idemPayment.Amount != req.Amount {
			http.Error(w, "Please create the newOrder", http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(idemPayment)
		return
	}

	payment := Payment{
		Id:      nextPaymentId,
		OrderId: req.OrderId,
		Amount:  req.Amount,
		Status:  cmd.StatusPending,
	}

	paymentMap[payment.Id] = payment
	idempotencyMap[key] = payment.Id
	nextPaymentId++
	go fakeProvider(payment.Id)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(payment)

}

func getPayment(w http.ResponseWriter, r *http.Request) {
	strid := r.PathValue("id")

	id, err := strconv.Atoi(strid)
	if err != nil {
		http.Error(w, "invalid payment id", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	pid, yes := paymentMap[id]
	if !yes {
		http.Error(w, "Please enter the correct payment Id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pid)

}

func webhookhandler(w http.ResponseWriter, r *http.Request) {

	var eventResponse webhookEvent

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invlaid Body", http.StatusBadRequest)
		return
	}
	got := r.Header.Get("X-Signature")
	expected := sign(body)

	if !hmac.Equal([]byte(got), []byte(expected)) {
		http.Error(w, "Invlaid Signature", http.StatusUnauthorized)
		return
	}
	if err := json.Unmarshal(body, &eventResponse); err != nil {
		http.Error(w, "Invlaid Payload", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	payment, ok := paymentMap[eventResponse.PaymentId]
	if !ok {
		http.Error(w, "Payment Not Found", http.StatusBadRequest)
		return
	}
	if payment.Status != cmd.StatusPending {
		w.WriteHeader(http.StatusOK)
		return
	}
	payment.Status = eventResponse.Status
	paymentMap[eventResponse.PaymentId] = payment

	w.WriteHeader(http.StatusOK)
}

func main() {
	mux := http.NewServeMux()

	go func() {
		lis, err := net.Listen("tcp", ":9092")
		if err != nil {
			log.Fatal(err)
		}
		s := grpc.NewServer()
		paymentpb.RegisterPaymentServiceServer(s, &paymentGRPCServer{})
		reflection.Register(s)
		log.Println("grpc Server on 9092")
		s.Serve(lis)
	}()

	mux.HandleFunc("POST /webhook", webhookhandler)
	mux.HandleFunc("POST /payments", createPayment)
	mux.HandleFunc("GET /payments/{id}", getPayment)
	log.Println("HTTP server on :8082")
	http.ListenAndServe(":8082", mux)
}
