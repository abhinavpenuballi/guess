package router

import (
	"guess/handlers"
	"net/http"
)

type RouterClient struct {
	HandlersClient *handlers.HandlersClient
}

func (c *RouterClient) GetMux() *http.ServeMux {
	get := http.NewServeMux()
	get.HandleFunc("/home", c.HandlersClient.Home)
	get.HandleFunc("/lobby", c.HandlersClient.Lobby)
	get.HandleFunc("/lobby_waiting", c.HandlersClient.LobbyWaiting)
	get.HandleFunc("/start", c.HandlersClient.Start)
	get.HandleFunc("/game", c.HandlersClient.Game)
	get.HandleFunc("/game_state", c.HandlersClient.GameState)
	get.HandleFunc("/move", c.HandlersClient.GetMove)
	get.HandleFunc("/over", c.HandlersClient.Over)

	post := http.NewServeMux()
	post.HandleFunc("/join", c.HandlersClient.Join)
	post.HandleFunc("/move", c.HandlersClient.PostMove)

	mux := (http.NewServeMux())
	mux.Handle("GET /", get)
	mux.Handle("POST /", post)

	return mux
}
