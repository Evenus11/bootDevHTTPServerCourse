package main

import (
	"net/http"
)

func main() {
	v := http.NewServeMux()

	v.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))
	v.HandleFunc("/healthz", healthzHandler)

	d := http.Server{
		Addr:    ":8080",
		Handler: v,
	}

	err := d.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK) // 200 status code
	w.Write([]byte("OK"))
}
