package server

import (
	"sync"
	"time"

	"github.com/mr1hm/go-uno/internal/game"
)

const (
	reconnectGrace = 30 * time.Second // Time to reconnect before being removed
)

// DisconnectedPlayer tracks players who may reconnect
type DisconnectedPlayer struct {
	PlayerID  string
	RoomID    string
	ExpiresAt time.Time
}

// Hub manages all connections and rooms
type Hub struct {
	rooms        map[string]*Room
	clients      map[*Client]bool
	disconnected map[string]*DisconnectedPlayer // playerID -> disconnect info
	register     chan *Client
	unregister   chan *Client
	stop         chan struct{}
	mu           sync.RWMutex
}

// Register adds a client to the hub
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Stop gracefully shuts down the hub
func (h *Hub) Stop() {
	close(h.stop)
}

// NewHub creates a new hub
func NewHub() *Hub {
	return &Hub{
		rooms:        make(map[string]*Room),
		clients:      make(map[*Client]bool),
		disconnected: make(map[string]*DisconnectedPlayer),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		stop:         make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	cleanupTicker := time.NewTicker(10 * time.Second)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-h.stop:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.handleDisconnect(client)

		case <-cleanupTicker.C:
			h.cleanupDisconnected()
			h.cleanupEmptyRooms()
		}
	}
}

// handleDisconnect manages player disconnection
func (h *Hub) handleDisconnect(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[c]; !ok {
		return
	}

	delete(h.clients, c)
	c.closed = true
	close(c.send)

	if c.RoomID == "" {
		return
	}

	room, ok := h.rooms[c.RoomID]
	if !ok {
		return
	}

	// If game started, add to disconnected list for potential reconnect
	if room.started {
		h.disconnected[c.ID] = &DisconnectedPlayer{
			PlayerID:  c.ID,
			RoomID:    c.RoomID,
			ExpiresAt: time.Now().Add(reconnectGrace),
		}
		// Remove from room clients so broadcast skips them
		room.RemoveClient(c)
		// Notify others
		room.Broadcast(&ServerMessage{
			Type:       MsgPlayerLeft,
			PlayerID:   c.ID,
			PlayerName: c.Name,
		})
	} else {
		// Game not started, remove from room
		room.RemoveClient(c)
		room.Broadcast(&ServerMessage{
			Type:       MsgPlayerLeft,
			PlayerID:   c.ID,
			PlayerName: c.Name,
		})
	}
}

// cleanupDisconnected removes expired disconnected players
func (h *Hub) cleanupDisconnected() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for id, dp := range h.disconnected {
		if now.After(dp.ExpiresAt) {
			delete(h.disconnected, id)
			// Could also remove from game/forfeit here
		}
	}
}

// cleanupEmptyRooms removes rooms with no players
func (h *Hub) cleanupEmptyRooms() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for id, room := range h.rooms {
		if room.IsEmpty() {
			delete(h.rooms, id)
		}
	}
}

// handleMessage processes incoming client messages
func (h *Hub) handleMessage(c *Client, msg *ClientMessage) {
	switch msg.Type {
	case MsgJoin:
		h.handleJoin(c, msg)
	case MsgPlayCard:
		h.handlePlayCard(c, msg)
	case MsgDrawCard:
		h.handleDrawCard(c)
	case MsgCallUno:
		h.handleCallUno(c)
	case MsgChallengeUno:
		h.handleChallengeUno(c, msg)
	case MsgPass:
		h.handlePass(c)
	}
}

// handleJoin handles room join/create and reconnection
func (h *Hub) handleJoin(c *Client, msg *ClientMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()

	c.Name = msg.PlayerName
	if c.Name == "" {
		c.Name = "Player"
	}

	roomID := msg.RoomID
	if roomID == "" {
		roomID = generateRoomID()
	}

	// Check for reconnection
	if dp, ok := h.disconnected[c.ID]; ok && dp.RoomID == roomID {
		delete(h.disconnected, c.ID)
		if room, ok := h.rooms[roomID]; ok {
			room.mu.Lock()
			room.clients[c.ID] = c
			c.RoomID = roomID
			room.mu.Unlock()

			// Send current state
			c.Send(room.buildStateMsg(c.ID))
			room.Broadcast(&ServerMessage{
				Type:       MsgPlayerJoined,
				PlayerID:   c.ID,
				PlayerName: c.Name,
			})
			return
		}
	}

	// Get or create room
	room, ok := h.rooms[roomID]
	if !ok {
		room = NewRoom(roomID)
		h.rooms[roomID] = room
	}

	if !room.AddClient(c) {
		c.sendError("room full or game started")
		return
	}

	// Notify client of room info
	c.Send(&ServerMessage{
		Type:     MsgRoomInfo,
		RoomID:   roomID,
		PlayerID: c.ID,
	})

	// Notify others
	room.Broadcast(&ServerMessage{
		Type:       MsgPlayerJoined,
		PlayerID:   c.ID,
		PlayerName: c.Name,
	})

	// Auto-start when enough players (for now, 2+)
	if room.CanStart() {
		room.StartGame()
		room.BroadcastState()
	}
}

// handlePlayCard processes a play card action
func (h *Hub) handlePlayCard(c *Client, msg *ClientMessage) {
	room := h.getRoom(c.RoomID)
	if room == nil {
		return
	}

	room.Lock()
	defer room.Unlock()

	g := room.game
	if g == nil || g.GameOver {
		return
	}

	chosenColor := game.Color(msg.ChosenColor)
	if err := g.PlayCard(c.ID, msg.CardIndex, chosenColor); err != nil {
		c.sendError(err.Error())
		return
	}

	room.BroadcastState()

	if g.GameOver && g.Winner != nil {
		room.Broadcast(&ServerMessage{
			Type:   MsgGameOver,
			Winner: g.Winner.Name,
		})
	}
}

// handleDrawCard processes a draw card action
func (h *Hub) handleDrawCard(c *Client) {
	room := h.getRoom(c.RoomID)
	if room == nil {
		return
	}

	room.Lock()
	defer room.Unlock()

	g := room.game
	if g == nil || g.GameOver {
		return
	}

	if _, err := g.DrawCard(c.ID); err != nil {
		c.sendError(err.Error())
		return
	}

	room.BroadcastState()
}

// handleCallUno processes UNO call
func (h *Hub) handleCallUno(c *Client) {
	room := h.getRoom(c.RoomID)
	if room == nil {
		return
	}

	room.Lock()
	defer room.Unlock()

	g := room.game
	if g == nil {
		return
	}

	if err := g.CallUno(c.ID); err != nil {
		c.sendError(err.Error())
		return
	}

	room.Broadcast(&ServerMessage{
		Type:       MsgCallUno,
		PlayerID:   c.ID,
		PlayerName: c.Name,
	})
}

// handleChallengeUno processes UNO challenge
func (h *Hub) handleChallengeUno(c *Client, msg *ClientMessage) {
	room := h.getRoom(c.RoomID)
	if room == nil {
		return
	}

	room.Lock()
	defer room.Unlock()

	g := room.game
	if g == nil {
		return
	}

	if err := g.ChallengeUno(c.ID, msg.TargetID); err != nil {
		c.sendError(err.Error())
		return
	}

	room.BroadcastState()
	room.Broadcast(&ServerMessage{
		Type:       MsgChallengeUno,
		PlayerID:   c.ID,
		PlayerName: c.Name,
		Action:     "caught " + msg.TargetID,
	})
}

// handlePass processes pass action
func (h *Hub) handlePass(c *Client) {
	room := h.getRoom(c.RoomID)
	if room == nil {
		return
	}

	room.Lock()
	defer room.Unlock()

	g := room.game
	if g == nil || g.GameOver {
		return
	}

	if err := g.PassTurn(c.ID); err != nil {
		c.sendError(err.Error())
		return
	}

	room.BroadcastState()
}

// getRoom returns a room by ID
func (h *Hub) getRoom(roomID string) *Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[roomID]
}

// generateRoomID creates a short room code
func generateRoomID() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
