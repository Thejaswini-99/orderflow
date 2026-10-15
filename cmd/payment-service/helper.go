package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

const webhook_secret = "my_secret_123"

func sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(webhook_secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

type webhookEvent struct {
	PaymentId int    `json:"payment_id"`
	Status    string `json:"status"`
}

func fakeProvider(paymentId int) {
	time.Sleep(3 * time.Second)

	statuses := []string{"Success", "Failed"}
	status := statuses[rand.IntN(len(statuses))]

	event := webhookEvent{
		PaymentId: paymentId,
		Status:    status,
	}
	body, _ := json.Marshal(event)

	req, _ := http.NewRequest("POST", "http://localhost:8082/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", sign(body))
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		fmt.Println("Webhook Failed", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Webhook Sent:", event.Status, "-> response", resp.StatusCode)
}
