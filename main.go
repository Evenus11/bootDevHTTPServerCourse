package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
)

func main() {
	v := http.NewServeMux()
	c := ApiConfig{}
	v.Handle("/app/", c.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	v.HandleFunc("GET /api/healthz", healthzHandler)
	v.HandleFunc("GET /admin/metrics", c.hitsHandler)
	v.HandleFunc("POST /admin/reset", c.resetHitsHandler)
	v.HandleFunc("POST /api/validate_chirp", c.validateChirp)

	d := http.Server{
		Addr:    ":8080",
		Handler: v,
	}

	err := d.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

func (cfg *ApiConfig) middlewareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.FileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

type ApiConfig struct {
	FileServerHits atomic.Int32
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK) // 200 status code
	w.Write([]byte("OK"))
}

func (cfg *ApiConfig) hitsHandler(w http.ResponseWriter, r *http.Request) {
	d := cfg.FileServerHits.Load()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK) // 200 status code
	w.Write([]byte(fmt.Sprintf(`<html>
	<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visited %d times!</p>
	</body>
	</html>`, d)))
}
func (cfg *ApiConfig) resetHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	cfg.FileServerHits.Store(0)
}

type validateChirpRequest struct {
	Body string `json:"body"`
}
type errorResponse struct {
	Error string `json:"error"`
}
type valid struct {
	Valid bool `json:"valid"`
}
type CleanedResponse struct {
	CleanedBody string `json:"cleaned_body"`
}

func (cfg *ApiConfig) validateChirp(w http.ResponseWriter, r *http.Request) {
	d := validateChirpRequest{}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&d)

	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	if len(d.Body) > 140 {
		respondWithError(w, 400, "Chirp too large")

	} else {
		s := badWordReplacement(d)
		err := respondWithJSON(w, 200, s)
		if err != nil {
			respondWithError(w, 500, err.Error())
		}
	}
}

var ProfaneWords = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
}

func badWordReplacement(v validateChirpRequest) CleanedResponse {
	s := strings.Split(v.Body, " ")
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(ProfaneWords); j++ {
			if strings.ToLower(s[i]) == strings.ToLower(ProfaneWords[j]) {
				s[i] = "****"
				break
			}
		}
	}
	result := CleanedResponse{
		CleanedBody: strings.Join(s, " "),
	}
	return result
}

func respondWithError(w http.ResponseWriter, code int, message string) error {
	w.WriteHeader(code)
	e := errorResponse{
		Error: message,
	}
	dat, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = w.Write(dat)
	if err != nil {
		return err
	}
	return nil
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	w.WriteHeader(code)
	dat, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = w.Write(dat)
	if err != nil {
		return err
	}
	return nil
}
