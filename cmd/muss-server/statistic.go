package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	ss "github.com/muss/muss-go/shadowsocks"
)

func processStatisticRequest(writer http.ResponseWriter, request *http.Request) {
	defer request.Body.Close()
	writer.WriteHeader(200)
	statData := ss.GetUserStatisticMap()
	data, err := json.Marshal(statData)
	if err != nil {
		log.Print(err)
		fmt.Fprintf(writer, "{}")
	} else {
		writer.Write(data)
	}
}

func StartStatisticServer(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", processStatisticRequest)
	var server = http.Server{
		Addr:           addr,
		Handler:        mux,
		ReadTimeout:    300 * time.Second,
		WriteTimeout:   300 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	defer func() {
		if err := recover(); err != nil {
			log.Print(err)
		}
	}()
	log.Printf("Start Statistic HTTP Server: %s\n", addr)
	err := server.ListenAndServe()
	if err != nil {
		log.Print("Cannot Start Statistic HTTP Server")
	}
}
