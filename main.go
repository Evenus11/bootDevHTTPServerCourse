package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/evenus11/bootDevHTTPServerCourse/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	v := http.NewServeMux()
	c := ApiConfig{
		database: dbQueries,
		platform: platform,
	}
	v.Handle("/app/", c.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	v.HandleFunc("GET /api/healthz", healthzHandler)
	v.HandleFunc("GET /admin/metrics", c.hitsHandler)
	v.HandleFunc("POST /admin/reset", c.resetHitsHandler)
	v.HandleFunc("POST /api/chirps", c.createChirp)
	v.HandleFunc("POST /api/users", c.createUser)

	d := http.Server{
		Addr:    ":8080",
		Handler: v,
	}

	err = d.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

type ApiConfig struct {
	FileServerHits atomic.Int32
	database       *database.Queries
	platform       string
}
type chirpCreation struct {
	Body   string `json:"body"`
	UserId string `json:"user_id"`
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

type userCreation struct {
	Email string `json:"email"`
}

type user struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Email     string `json:"email"`
}

func (cfg *ApiConfig) middlewareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.FileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
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
	if cfg.platform == "dev" {
		err := cfg.database.DeleteUsers(r.Context())
		if err != nil {
			log.Fatal(err)
			w.WriteHeader(501)
		}
	}

}

var ProfaneWords = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
}

func badWordReplacement(v database.CreateChirpParams) CleanedResponse {
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

func (cfg *ApiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	uc := userCreation{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&uc)
	if err != nil {
		log.Println(err)
	}
	defer r.Body.Close()

	u, err := cfg.database.CreateUsers(r.Context(), uc.Email)
	if err != nil {
		log.Println(err)
	}
	responseUser := user{
		ID:        u.ID.String(), // assuming ID is uuid.UUID
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
		UpdatedAt: u.UpdatedAt.Format(time.RFC3339),
		Email:     u.Email,
	}
	err = respondWithJSON(w, 201, responseUser)
	if err != nil {
		respondWithError(w, 500, err.Error())
	}

}

func (cfg *ApiConfig) createChirp(w http.ResponseWriter, r *http.Request) {

	d := database.CreateChirpParams{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&d)
	fmt.Println(d)
	if err != nil {
		respondWithError(w, 400, err.Error())
		return
	}
	defer r.Body.Close()

	if len(d.Body) > 140 {
		respondWithError(w, 400, "Chirp too large")
		return

	} else {
		s := badWordReplacement(d)

		d.Body = s.CleanedBody
		c, err := cfg.database.CreateChirp(r.Context(), d)
		if err != nil {
			respondWithError(w, 500, err.Error())
			return
		}

		err = respondWithJSON(w, 201, c)
		if err != nil {
			respondWithError(w, 501, err.Error())
			return
		}

	}
}
