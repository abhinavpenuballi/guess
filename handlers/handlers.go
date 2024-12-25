package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

var mu sync.Mutex

type HandlersClient struct {
	RDB *redis.Client
}

func (c *HandlersClient) Home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "htmls/home.html")
}

func (c *HandlersClient) Join(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	name := r.PostFormValue("name")
	gameID := r.PostFormValue("game_id")

	N := 4
	MAX := math.Pow10(N) //exclusive
	MIN := 1             //inclusive

	if gameID == "" {
		for {
			gameID = fmt.Sprint(rand.IntN(int(MAX)-MIN) + MIN)

			res, err := c.RDB.HGet(ctx, "games", gameID).Result()
			if err != nil && err.Error() != "redis: nil" {
				w.WriteHeader(http.StatusInternalServerError)
				log.Panicln(err)
			}
			if res == "" {
				err := c.RDB.HSet(ctx, "games", gameID, "created").Err()
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					log.Panicln(err)
				}
				break
			}
		}
	} else {
		res, err := c.RDB.HGet(ctx, "games", gameID).Result()
		if err != nil && err.Error() != "redis: nil" {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		if res != "created" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("game not found"))
			return
		}
	}

	res, err := c.RDB.HGet(ctx, "game_"+gameID+"_player_names", name).Result()
	if err != nil && err.Error() != "redis: nil" {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	playerID := ""
	if res == "" {
		mu.Lock()

		res, err := c.RDB.HLen(ctx, "game_"+gameID+"_player_names").Result()
		if err != nil && err.Error() != "redis: nil" {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}

		err = c.RDB.HSet(ctx, "game_"+gameID+"_player_names", name, "").Err()
		if err != nil && err.Error() != "redis: nil" {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}

		playerID = fmt.Sprint(res + 1)

		err = c.RDB.HSet(ctx, "game_"+gameID+"_players", playerID, name).Err()
		if err != nil && err.Error() != "redis: nil" {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}

		mu.Unlock()
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("player already exists"))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:  "name",
		Value: name,
	})
	http.SetCookie(w, &http.Cookie{
		Name:  "id",
		Value: playerID,
	})
	http.SetCookie(w, &http.Cookie{
		Name:  "game_id",
		Value: gameID,
	})

	http.Redirect(w, r, "/lobby", http.StatusFound)
}

func (c *HandlersClient) Lobby(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "htmls/lobby.html")
}

func (c *HandlersClient) LobbyWaiting(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gameIDCookie, err := r.Cookie("game_id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	gameID := gameIDCookie.Value

	{
		res, err := c.RDB.HGet(ctx, "games", gameID).Result()
		if err != nil && err.Error() != "redis: nil" {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		if res == "started" {
			http.Redirect(w, r, "/game", http.StatusFound)
			return
		}
		if res != "created" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("game not found"))
			return
		}
	}

	res, err := c.RDB.HGetAll(ctx, "game_"+gameID+"_players").Result()
	if err != nil && err.Error() != "redis: nil" {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	out, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	w.Write(out)
}

func (c *HandlersClient) Start(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gameIDCookie, err := r.Cookie("game_id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	gameID := gameIDCookie.Value

	{
		res, err := c.RDB.HGet(ctx, "games", gameID).Result()
		if err != nil && err.Error() != "redis: nil" {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		if res != "created" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("game not found"))
			return
		}
	}

	err = c.RDB.HSet(ctx, "games", gameID, "started").Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	err = c.RDB.Set(ctx, "game_"+gameID+"_now_playing", "1", 0).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	err = c.RDB.Set(ctx, "game_"+gameID+"_min", "1", 0).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	err = c.RDB.Set(ctx, "game_"+gameID+"_max", "100", 0).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	number := rand.IntN(100) + 1
	err = c.RDB.Set(ctx, "game_"+gameID+"_number", number, 0).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	http.Redirect(w, r, "/game", http.StatusFound)
}

func (c *HandlersClient) Game(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "htmls/game.html")
}

type GameData struct {
	Min        string           `json:"min"`
	Max        string           `json:"max"`
	NowPlaying string           `json:"now_playing"`
	Players    [][3]interface{} `json:"players"` // ID, Name, Last Entered
}

func (c *HandlersClient) GameState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gameIDCookie, err := r.Cookie("game_id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	gameID := gameIDCookie.Value

	gameData := GameData{}
	{
		res, err := c.RDB.Get(ctx, "game_"+gameID+"_min").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		gameData.Min = res
	}
	{
		res, err := c.RDB.Get(ctx, "game_"+gameID+"_max").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		gameData.Max = res
	}
	{
		res, err := c.RDB.Get(ctx, "game_"+gameID+"_now_playing").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		gameData.NowPlaying = res
	}
	{
		resPlayerIDToName, err := c.RDB.HGetAll(ctx, "game_"+gameID+"_players").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		resPlayerNameToLastEntered, err := c.RDB.HGetAll(ctx, "game_"+gameID+"_player_names").Result()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		for numPlayers, i := len(resPlayerIDToName), 1; i <= numPlayers; i++ {
			id := fmt.Sprint(i)
			name := resPlayerIDToName[id]
			gameData.Players = append(gameData.Players, [3]interface{}{
				id,
				name,
				resPlayerNameToLastEntered[name],
			})
		}
	}

	out, err := json.Marshal(gameData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	w.Write(out)
}

func (c *HandlersClient) GetMove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gameIDCookie, err := r.Cookie("game_id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	gameID := gameIDCookie.Value

	msg := <-c.RDB.Subscribe(ctx, "game_"+gameID+"_channel").Channel()
	str := msg.String()

	length := len(str)

	if str := str[length-5 : length-1]; str == "move" {
		http.Redirect(w, r, "/game", http.StatusFound)
	} else if str == "over" {
		http.Redirect(w, r, "/over", http.StatusFound)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panic("unexpected message", str, "received on channel")
	}
}

func (c *HandlersClient) PostMove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gameIDCookie, err := r.Cookie("game_id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	gameID := gameIDCookie.Value

	nameCookie, err := r.Cookie("name")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	name := nameCookie.Value

	idCookie, err := r.Cookie("id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	id := idCookie.Value

	nowPlaying, err := c.RDB.Get(ctx, "game_"+gameID+"_now_playing").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	if id != nowPlaying {
		w.Write([]byte("not your turn"))
		return
	}

	err = r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	inputNum := r.PostFormValue("input")

	inputNumInt, err := strconv.Atoi(inputNum)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	actualNum, err := c.RDB.Get(ctx, "game_"+gameID+"_number").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	actualNumInt, err := strconv.Atoi(actualNum)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	if inputNumInt < actualNumInt {
		err = c.RDB.Set(ctx, "game_"+gameID+"_min", inputNumInt+1, 0).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
	} else if inputNumInt > actualNumInt {
		err = c.RDB.Set(ctx, "game_"+gameID+"_max", inputNumInt-1, 0).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
	} else {
		err = c.RDB.Set(ctx, "game_"+gameID+"_min", inputNum, 0).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		err = c.RDB.Set(ctx, "game_"+gameID+"_max", inputNum, 0).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		err = c.RDB.Set(ctx, "game_"+gameID+"_over", name, 0).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		err = c.RDB.Del(ctx,
			"game_"+gameID+"_players",
			"game_"+gameID+"_player_names",
			"game_"+gameID+"_min",
			"game_"+gameID+"_max",
			"game_"+gameID+"_now_playing",
		).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}
		err = c.RDB.HDel(ctx, "games", gameID).Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}

		err = c.RDB.Publish(ctx, "game_"+gameID+"_channel", "over").Err()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Panicln(err)
		}

		http.Redirect(w, r, "/over", http.StatusFound)
	}

	err = c.RDB.HSet(ctx, "game_"+gameID+"_player_names", name, inputNum).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	nowPlayingInt, err := strconv.Atoi(nowPlaying)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	nowPlayingInt++

	numPlayers, err := c.RDB.HLen(ctx, "game_"+gameID+"_players").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	if nowPlayingInt > int(numPlayers) {
		nowPlayingInt = 1
	}

	err = c.RDB.Set(ctx, "game_"+gameID+"_now_playing", nowPlayingInt, 0).Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	err = c.RDB.Publish(ctx, "game_"+gameID+"_channel", "move").Err()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	http.Redirect(w, r, "/game", http.StatusFound)
}

func (c *HandlersClient) Over(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gameIDCookie, err := r.Cookie("game_id")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}
	gameID := gameIDCookie.Value

	bytes, err := os.ReadFile("htmls/over.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	number, err := c.RDB.Get(ctx, "game_"+gameID+"_number").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	name, err := c.RDB.Get(ctx, "game_"+gameID+"_over").Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Panicln(err)
	}

	bytes = []byte(strings.Replace(string(bytes), `[%%NUMBER%%]`, number, -1))
	bytes = []byte(strings.Replace(string(bytes), `[%%PLAYER%%]`, name, -1))
	w.Write(bytes)
}
