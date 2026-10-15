package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/Thejaswini-99/orderflow/proto/inventorypb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Product struct {
	Id    int    `json:"product_id"`
	Name  string `json:"product_name"`
	Price int    `json:"price"`
	Stock int    `json:"stock"`
}

var products = []Product{
	{1, "Laptop", 50000, 20},
	{2, "Televison", 25000, 30},
	{3, "Mouse", 3500, 20},
}

func main() {
	mux := http.NewServeMux()

	go func() {

		lis, err := net.Listen("tcp", ":9093")
		if err != nil {
			panic(err)
		}
		s := grpc.NewServer()
		inventorypb.RegisterInventoryServiceServer(s, &inventoryGRPCServer{})
		reflection.Register(s)
		log.Println("Grpc server on 9093")
		if err := s.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	mux.HandleFunc("GET /products", getProducts)
	mux.HandleFunc("GET /product/{id}", getProductbyId)

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}

func getProductbyId(w http.ResponseWriter, r *http.Request) {

	strid := r.PathValue("id")
	mu.Lock()
	defer mu.Unlock()
	id, err := strconv.Atoi(strid)
	if err != nil {
		http.Error(w, "Invalid Id", http.StatusBadRequest)
		return
	}

	for i, val := range products {
		if id == val.Id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(products[i])
			return
		}
	}

	http.Error(w, "No Such Products Present", http.StatusNotFound)

}

func getProducts(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}
