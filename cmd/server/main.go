package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/mr1hm/go-uno/internal/server"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for Discord Activity iframe
		return true
	},
}

func main() {
	hub := server.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}

		// Get player ID from query or generate one
		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			playerID = generateID()
		}

		client := server.NewClient(hub, conn, playerID)
		hub.Register(client)

		go client.WritePump()
		go client.ReadPump()
	})

	// Discord OAuth token exchange
	http.HandleFunc("/api/token", handleTokenExchange)

	// Serve index.html with Discord client ID injected
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			serveIndex(w, r)
			return
		}
		http.FileServer(http.Dir("web")).ServeHTTP(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	clientID := os.Getenv("DISCORD_CLIENT_ID")
	if clientID == "" {
		clientID = "YOUR_CLIENT_ID"
	}

	html := strings.Replace(string(data), "{{DISCORD_CLIENT_ID}}", clientID, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func handleTokenExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	clientID := os.Getenv("DISCORD_CLIENT_ID")
	clientSecret := os.Getenv("DISCORD_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Println("DISCORD_CLIENT_ID or DISCORD_CLIENT_SECRET not set")
		http.Error(w, "Server misconfigured", http.StatusInternalServerError)
		return
	}

	// Exchange code for token with Discord
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", req.Code)

	resp, err := http.Post(
		"https://discord.com/api/oauth2/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		log.Printf("Discord token exchange failed: %v", err)
		http.Error(w, "Token exchange failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Discord returned status %d", resp.StatusCode)
		http.Error(w, "Token exchange failed", http.StatusBadGateway)
		return
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		log.Printf("Failed to decode Discord response: %v", err)
		http.Error(w, "Token exchange failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": tokenResp.AccessToken,
	})
}
