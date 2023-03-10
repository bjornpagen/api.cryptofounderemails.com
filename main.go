package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	stripe "github.com/stripe/stripe-go"
	"golang.org/x/crypto/acme/autocert"

	"github.com/bjornpagen/e2e-marketing-monorepo/server/lookup"
)

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file, using environment variables instead.")
	}

	// check for required environment variables
	if os.Getenv("ID_DB") == "" {
		log.Fatalln("ID_DB environment variable not set.")
	}

	if os.Getenv("CLIENT_DOMAIN") == "" {
		log.Fatalln("CLIENT_DOMAIN environment variable not set.")
	}

	if os.Getenv("API_DOMAIN") == "" {
		log.Fatalln("API_DOMAIN environment variable not set.")
	}

	if os.Getenv("TLS_DISABLED") == "" {
		log.Fatalln("TLS_DISABLED environment variable not set.")
	}

	if os.Getenv("STRIPE_KEY") == "" {
		log.Fatalln("STRIPE_KEY environment variable not set.")
	}

	if os.Getenv("STRIPE_WEBHOOK_SECRET") == "" {
		log.Fatalln("STRIPE_WEBHOOK_SECRET environment variable not set.")
	}
}

func loadDb() (map[lookup.Id]lookup.User, error) {
	db := make(map[lookup.Id]lookup.User)

	b, err := ioutil.ReadFile(os.Getenv("ID_DB"))
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &db); err != nil {
		return nil, err
	}

	return db, nil
}

func main() {
	loadEnv()

	lookupDb, err := loadDb()
	if err != nil {
		log.Fatalln("Error loading id db:", err)
	}

	server := &MainServer{
		apiDomain:            os.Getenv("API_DOMAIN"),
		lookup:               lookup.New(lookupDb, log.New(os.Stderr, "lookup: ", log.LstdFlags), os.Getenv("CLIENT_DOMAIN")),
		tlsDisabled:          os.Getenv("TLS_DISABLED") == "true",
		stripeKey:            os.Getenv("STRIPE_KEY"),
		stripeWebhookHandler: createStripeWebhookHandler(os.Getenv("STRIPE_WEBHOOK_SECRET")),
	}

	if err := server.run(); err != nil {
		log.Fatalln("Error running server:", err)
	}
}

type MainServer struct {
	stripeKey            string
	stripeWebhookHandler http.HandlerFunc
	apiDomain            string
	lookup               *lookup.LookupClient
	tlsDisabled          bool
}

func (s *MainServer) run() (err error) {
	// setup stripe
	stripe.Key = s.stripeKey

	// setup router
	r := setupRouter()

	r.Post("/lookup", s.lookup.LookupHandler)
	r.Options("/lookup", s.lookup.OptionsHandler)
	r.Post("/stripe/webhook", s.stripeWebhookHandler)

	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      r,
	}

	// start server
	if s.tlsDisabled {
		log.Println("TLS disabled, starting server on port 8080")
		srv.Addr = ":8080"

		err = srv.ListenAndServe()
	} else {
		log.Println("TLS enabled, starting server on port 443")
		srv.Addr = ":443"

		err = srv.Serve(autocert.NewListener(s.apiDomain))
	}

	return err
}

func setupRouter() (r *chi.Mux) {
	r = chi.NewRouter()
	r.Use(middleware.Logger)

	// Enable httprate request limiter of 100 requests per minute.
	//
	// In the code example below, rate-limiting is bound to the request IP address
	// via the LimitByIP middleware handler.
	//
	// To have a single rate-limiter for all requests, use httprate.LimitAll(..).
	//
	// Please see _example/main.go for other more, or read the library code.
	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	return r
}
