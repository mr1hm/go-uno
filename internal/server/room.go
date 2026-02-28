package server

import (
	"sync"

	"github.com/mr1hm/go-uno/internal/game"
)

const (
	MinPlayers = 2
	MaxPlayers = 8
)

// Room represents a game room
type Room struct {
	ID      string
	clients map[string]*Client // playerID -> client
	game    *game.GameState
	order   []string // Player IDs in join order
	mu      sync.RWMutex
	started bool
}

// NewRoom creates a new room
func NewRoom(id string) *Room {
	return &Room{
		ID:      id,
		clients: make(map[string]*Client),
		order:   make([]string, 0, MaxPlayers),
	}
}

// AddClient adds a player to the room
func (r *Room) AddClient(c *Client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started || len(r.clients) >= MaxPlayers {
		return false
	}

	r.clients[c.ID] = c
	r.order = append(r.order, c.ID)
	c.RoomID = r.ID
	return true
}

// RemoveClient removes a player from the room
func (r *Room) RemoveClient(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.clients, c.ID)
	for i, id := range r.order {
		if id == c.ID {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}

// ClientCount returns number of players
func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// CanStart returns true if game can begin
func (r *Room) CanStart() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return !r.started && len(r.clients) >= MinPlayers
}

// StartGame initializes the game state
func (r *Room) StartGame() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started || len(r.clients) < MinPlayers {
		return false
	}

	names := make([]string, len(r.order))
	for i, id := range r.order {
		if c, ok := r.clients[id]; ok {
			names[i] = c.Name
		}
	}

	r.game = game.NewGame(names)
	if r.game == nil {
		return false
	}

	// Map game player IDs to client IDs
	for i, id := range r.order {
		r.game.Players[i].ID = id
	}

	r.started = true
	return true
}

// GetPlayerIndex returns the index of a player by ID
func (r *Room) GetPlayerIndex(playerID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i, id := range r.order {
		if id == playerID {
			return i
		}
	}
	return -1
}

// Broadcast sends a message to all clients
func (r *Room) Broadcast(msg *ServerMessage) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, c := range r.clients {
		c.Send(msg)
	}
}

// BroadcastState sends game state to each player (with their own hand)
func (r *Room) BroadcastState() {
	// NOTE: Do NOT hold room lock here, caller already has it
	if r.game == nil {
		return
	}

	for _, c := range r.clients {
		c.Send(r.buildStateMsg(c.ID))
	}
}

// buildStateMsg creates a state message for a specific player
func (r *Room) buildStateMsg(playerID string) *ServerMessage {
	g := r.game

	players := make([]PlayerInfo, len(g.Players))
	for i, p := range g.Players {
		players[i] = PlayerInfo{
			ID:           p.ID,
			Name:         p.Name,
			CardCount:    p.HandSize(),
			HasCalledUno: p.HasCalledUno,
		}
	}

	var hand []CardInfo
	if p := g.GetPlayerByID(playerID); p != nil {
		hand = CardsToInfo(p.Hand)
	}

	topCard := ToCardInfo(g.CurrentCard())

	return &ServerMessage{
		Type:          MsgGameState,
		Players:       players,
		CurrentPlayer: g.CurrentPlayer,
		Direction:     int(g.Direction),
		TopCard:       &topCard,
		ChosenColor:   int(g.ChosenColor),
		Hand:          hand,
	}
}

// IsEmpty returns true if room has no clients
func (r *Room) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients) == 0
}

// Game returns the game state (for action handling)
func (r *Room) Game() *game.GameState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.game
}

// Lock/Unlock for action handling
func (r *Room) Lock()   { r.mu.Lock() }
func (r *Room) Unlock() { r.mu.Unlock() }
