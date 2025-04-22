package main

import (
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"proxy-scanner/api"
	"proxy-scanner/proxy"
)

func main() {
	dsn := "postgres://proxyuser:proxypass@postgres:5432/proxydb?sslmode=disable"

	store, err := proxy.NewDBStore(dsn)

	if err != nil {
		log.Fatal("DB connection failed:", err)
	}

	certManager, err := proxy.NewCertManager("certs")
	if err != nil {
		log.Fatal("CertManager init failed:", err)
	}

	proxyHandler := proxy.NewProxyHandler(store, certManager)

	go func() {
		log.Println("Proxy server starting on :8080")
		if err := http.ListenAndServe(":8080", proxyHandler); err != nil {
			log.Fatal("Proxy server error:", err)
		}
	}()

	apiHandler := api.NewAPIHandler(store, proxyHandler)
	router := api.NewRouter(apiHandler)
	log.Printf("yobani API server starting on :8000 %+v", store)
	if err := http.ListenAndServe(":8000", router); err != nil {
		log.Fatal("API server error:", err)
	}
}
