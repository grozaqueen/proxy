package main

import (
	"log"
	"net/http"
	"proxy-scanner/api"
	"proxy-scanner/proxy"
)

func main() {
	store := proxy.NewRequestStore()

	certManager, err := proxy.NewCertManager("certs")
	if err != nil {
		log.Fatal("CertManager init failed:", err)
	}

	// Создание прокси с поддержкой MITM
	proxyHandler := proxy.NewProxyHandler(store, certManager)

	go func() {
		log.Println("Proxy server starting on :8080")
		if err := http.ListenAndServe(":8080", proxyHandler); err != nil {
			log.Fatal("Proxy server error:", err)
		}
	}()

	apiHandler := api.NewAPIHandler(store)
	router := api.NewRouter(apiHandler)
	log.Println("API server starting on :8000")
	if err := http.ListenAndServe(":8000", router); err != nil {
		log.Fatal("API server error:", err)
	}
}
