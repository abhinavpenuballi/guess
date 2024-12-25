package main

import (
	"guess/handlers"
	"guess/router"
	"log"
	"net/http"

	redis "github.com/redis/go-redis/v9"
)

func main() {
	handlersClient := &handlers.HandlersClient{
		RDB: redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		}),
	}

	routerClient := router.RouterClient{
		HandlersClient: handlersClient,
	}

	mux := logMW(routerClient.GetMux())

	server := http.Server{
		Addr:    ":8000",
		Handler: mux,
	}
	server.ListenAndServe()
}

func logMW(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}
