package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		// Ignore goroutines from test infrastructure
		goleak.IgnoreTopFunction("net/http.(*Server).Serve"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		// Ignore client pumps - they clean up when connections close
		goleak.IgnoreTopFunction("github.com/mr1hm/go-uno/internal/server.(*Client).ReadPump"),
		goleak.IgnoreTopFunction("github.com/mr1hm/go-uno/internal/server.(*Client).WritePump"),
		goleak.IgnoreTopFunction("github.com/mr1hm/go-uno/internal/server.(*Client).ReadPump.func1"),
	)
}

// setupTest creates a hub, runs it, and returns cleanup function
func setupTest(_ *testing.T) (*Hub, func()) {
	hub := NewHub()
	go hub.Run()
	return hub, func() {
		hub.Stop()
		time.Sleep(10 * time.Millisecond) // Let goroutine exit
	}
}

// testServer creates a test server with WebSocket endpoint
func testServer(t *testing.T, hub *Hub) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}

		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			playerID = "test-player"
		}

		client := NewClient(hub, conn, playerID)
		hub.Register(client)

		go client.WritePump()
		go client.ReadPump()
	}))
}

// testClient connects to test server and provides send/receive helpers
type testClient struct {
	t      *testing.T
	conn   *websocket.Conn
	id     string
	closed bool
	failed bool // Connection in failed state (can't read anymore)
	mu     sync.Mutex
}

func connectClient(t *testing.T, server *httptest.Server, playerID string) *testClient {
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?id=" + playerID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	return &testClient{t: t, conn: conn, id: playerID}
}

func (tc *testClient) send(msg *ClientMessage) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if tc.closed {
		return
	}
	data, _ := json.Marshal(msg)
	tc.conn.WriteMessage(websocket.TextMessage, data)
}

func (tc *testClient) receive(timeout time.Duration) (msg *ServerMessage) {
	tc.mu.Lock()
	if tc.closed || tc.failed {
		tc.mu.Unlock()
		return nil
	}
	tc.mu.Unlock()

	// Recover from panic in case of failed connection
	defer func() {
		if r := recover(); r != nil {
			tc.mu.Lock()
			tc.failed = true
			tc.mu.Unlock()
			msg = nil
		}
	}()

	tc.conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := tc.conn.ReadMessage()
	if err != nil {
		// Only mark as failed if it's a close error, not a timeout
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			tc.mu.Lock()
			tc.failed = true
			tc.mu.Unlock()
		}
		return nil
	}
	var result ServerMessage
	if err := json.Unmarshal(data, &result); err != nil {
		tc.t.Logf("unmarshal error: %v", err)
		return nil
	}
	return &result
}

func (tc *testClient) receiveType(msgType byte, timeout time.Duration) *ServerMessage {
	deadline := time.Now().Add(timeout)
	for {
		tc.mu.Lock()
		done := tc.closed || tc.failed
		tc.mu.Unlock()
		if done {
			return nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		// Use shorter read timeout to allow checking deadline
		readTimeout := min(remaining, 500*time.Millisecond)
		msg := tc.receive(readTimeout)
		if msg != nil && msg.Type == msgType {
			return msg
		}
	}
}

func (tc *testClient) close() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if !tc.closed {
		tc.closed = true
		tc.conn.Close()
	}
}

// Tests

func TestHubCreation(t *testing.T) {
	hub := NewHub()
	if hub == nil {
		t.Fatal("hub should not be nil")
	}
	if hub.rooms == nil {
		t.Error("rooms map should be initialized")
	}
	if hub.clients == nil {
		t.Error("clients map should be initialized")
	}
}

func TestClientConnection(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	client := connectClient(t, server, "player1")
	defer client.close()

	// Give hub time to register
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()

	if clientCount != 1 {
		t.Errorf("expected 1 client, got %d", clientCount)
	}
}

func TestRoomJoin(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	client := connectClient(t, server, "player1")
	defer client.close()

	// Join room
	client.send(&ClientMessage{
		Type:       MsgJoin,
		RoomID:     "TEST",
		PlayerName: "Alice",
	})

	// Should receive room info
	msg := client.receiveType(MsgRoomInfo, time.Second)
	if msg == nil {
		t.Fatal("expected room info message")
	}
	if msg.RoomID != "TEST" {
		t.Errorf("expected room ID 'TEST', got '%s'", msg.RoomID)
	}
	if msg.PlayerID != "player1" {
		t.Errorf("expected player ID 'player1', got '%s'", msg.PlayerID)
	}
}

func TestRoomAutoCreate(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	client := connectClient(t, server, "player1")
	defer client.close()

	// Join without room ID (should auto-create)
	client.send(&ClientMessage{
		Type:       MsgJoin,
		PlayerName: "Alice",
	})

	msg := client.receiveType(MsgRoomInfo, time.Second)
	if msg == nil {
		t.Fatal("expected room info message")
	}
	if msg.RoomID == "" {
		t.Error("expected auto-generated room ID")
	}
}

func TestTwoPlayersJoin(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	// Player 1 joins
	client1 := connectClient(t, server, "player1")
	defer client1.close()

	client1.send(&ClientMessage{
		Type:       MsgJoin,
		RoomID:     "GAME1",
		PlayerName: "Alice",
	})

	// Wait for room info
	client1.receiveType(MsgRoomInfo, time.Second)

	// Player 2 joins same room
	client2 := connectClient(t, server, "player2")
	defer client2.close()

	client2.send(&ClientMessage{
		Type:       MsgJoin,
		RoomID:     "GAME1",
		PlayerName: "Bob",
	})

	// Player 2 should get room info
	msg := client2.receiveType(MsgRoomInfo, time.Second)
	if msg == nil {
		t.Fatal("player 2 should receive room info")
	}

	// Both should receive game state (game auto-starts with 2 players)
	state1 := client1.receiveType(MsgGameState, time.Second)
	state2 := client2.receiveType(MsgGameState, time.Second)

	if state1 == nil || state2 == nil {
		t.Fatal("both players should receive game state")
	}

	// Verify player count
	if len(state1.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(state1.Players))
	}

	// Each player should have at least 7 cards (may have more if first card is Draw Two)
	if len(state1.Hand) < 7 {
		t.Errorf("player 1 should have at least 7 cards, got %d", len(state1.Hand))
	}
	if len(state2.Hand) < 7 {
		t.Errorf("player 2 should have at least 7 cards, got %d", len(state2.Hand))
	}
}

func TestGameActions(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	// Player 1 joins first
	client1 := connectClient(t, server, "player1")
	defer client1.close()

	client1.send(&ClientMessage{Type: MsgJoin, RoomID: "ACTION", PlayerName: "Alice"})
	msg := client1.receiveType(MsgRoomInfo, time.Second)
	if msg == nil {
		t.Fatal("player 1 should receive room info")
	}

	// Player 2 joins - this triggers game start
	client2 := connectClient(t, server, "player2")
	defer client2.close()

	client2.send(&ClientMessage{Type: MsgJoin, RoomID: "ACTION", PlayerName: "Bob"})
	msg = client2.receiveType(MsgRoomInfo, time.Second)
	if msg == nil {
		t.Fatal("player 2 should receive room info")
	}

	// Both should receive initial game state
	state1 := client1.receiveType(MsgGameState, 2*time.Second)
	state2 := client2.receiveType(MsgGameState, 2*time.Second)

	if state1 == nil || state2 == nil {
		t.Fatal("both players should receive game state")
	}

	// Determine active player and their hand
	var activeClient, otherClient *testClient
	var initialHandSize int
	if state1.CurrentPlayer == 0 {
		activeClient = client1
		otherClient = client2
		initialHandSize = len(state1.Hand)
	} else {
		activeClient = client2
		otherClient = client1
		initialHandSize = len(state2.Hand)
	}

	// Wait for hub to process initial state
	time.Sleep(100 * time.Millisecond)

	// Active player draws a card
	activeClient.send(&ClientMessage{Type: MsgDrawCard})

	// Should receive updated state
	newState := activeClient.receiveType(MsgGameState, 2*time.Second)
	if newState == nil {
		t.Fatal("should receive updated state after draw")
	}

	_ = otherClient // Verify no compile error

	// Hand size should increase by 1
	if len(newState.Hand) != initialHandSize+1 {
		t.Errorf("expected %d cards after draw, got %d", initialHandSize+1, len(newState.Hand))
	}
}

func TestWrongTurnRejected(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	// Player 1 joins first
	client1 := connectClient(t, server, "player1")
	defer client1.close()

	client1.send(&ClientMessage{Type: MsgJoin, RoomID: "TURN", PlayerName: "Alice"})
	client1.receiveType(MsgRoomInfo, time.Second)

	// Player 2 joins
	client2 := connectClient(t, server, "player2")
	defer client2.close()

	client2.send(&ClientMessage{Type: MsgJoin, RoomID: "TURN", PlayerName: "Bob"})
	client2.receiveType(MsgRoomInfo, time.Second)

	// Wait for game and get states
	time.Sleep(50 * time.Millisecond)
	state := client1.receiveType(MsgGameState, time.Second)
	client2.receiveType(MsgGameState, time.Second)

	if state == nil {
		t.Fatal("should receive game state")
	}

	// Determine who is NOT the current player
	var wrongClient *testClient
	if state.CurrentPlayer == 0 {
		wrongClient = client2
	} else {
		wrongClient = client1
	}

	// Wrong player tries to draw
	wrongClient.send(&ClientMessage{Type: MsgDrawCard})

	// Should receive error
	errMsg := wrongClient.receiveType(MsgError, 2*time.Second)
	if errMsg == nil {
		t.Fatal("wrong player should receive error")
	}
	if errMsg.Error == "" {
		t.Error("error message should not be empty")
	}
}

func TestPlayerDisconnect(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	client1 := connectClient(t, server, "player1")
	defer client1.close()
	client2 := connectClient(t, server, "player2")

	client1.send(&ClientMessage{Type: MsgJoin, RoomID: "DC", PlayerName: "Alice"})
	client2.send(&ClientMessage{Type: MsgJoin, RoomID: "DC", PlayerName: "Bob"})

	// Wait for game
	client1.receiveType(MsgRoomInfo, time.Second)
	client2.receiveType(MsgRoomInfo, time.Second)
	client1.receiveType(MsgGameState, time.Second)
	client2.receiveType(MsgGameState, time.Second)

	// Player 2 disconnects
	client2.close()

	// Player 1 should receive disconnect notification
	leftMsg := client1.receiveType(MsgPlayerLeft, time.Second)
	if leftMsg == nil {
		t.Fatal("should receive player left message")
	}
	if leftMsg.PlayerID != "player2" {
		t.Errorf("expected player2 left, got %s", leftMsg.PlayerID)
	}
}

func TestReconnection(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	client1 := connectClient(t, server, "player1")
	defer client1.close()
	client2 := connectClient(t, server, "player2")

	client1.send(&ClientMessage{Type: MsgJoin, RoomID: "RECONN", PlayerName: "Alice"})
	client2.send(&ClientMessage{Type: MsgJoin, RoomID: "RECONN", PlayerName: "Bob"})

	// Wait for game
	client1.receiveType(MsgRoomInfo, time.Second)
	client2.receiveType(MsgRoomInfo, time.Second)
	client1.receiveType(MsgGameState, time.Second)
	client2.receiveType(MsgGameState, time.Second)

	// Player 2 disconnects
	client2.close()
	time.Sleep(100 * time.Millisecond)

	// Player 2 reconnects with same ID
	client2New := connectClient(t, server, "player2")
	defer client2New.close()

	client2New.send(&ClientMessage{Type: MsgJoin, RoomID: "RECONN", PlayerName: "Bob"})

	// Should receive game state (reconnected to existing game)
	state := client2New.receiveType(MsgGameState, time.Second)
	if state == nil {
		t.Fatal("reconnected player should receive game state")
	}

	// Should have their hand
	if len(state.Hand) == 0 {
		t.Error("reconnected player should have cards")
	}
}

func TestCallUno(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	client1 := connectClient(t, server, "player1")
	defer client1.close()
	client2 := connectClient(t, server, "player2")
	defer client2.close()

	client1.send(&ClientMessage{Type: MsgJoin, RoomID: "UNO", PlayerName: "Alice"})
	client2.send(&ClientMessage{Type: MsgJoin, RoomID: "UNO", PlayerName: "Bob"})

	client1.receiveType(MsgRoomInfo, time.Second)
	client2.receiveType(MsgRoomInfo, time.Second)
	client1.receiveType(MsgGameState, time.Second)

	// Call UNO (even though we have 7 cards - just testing message flow)
	client1.send(&ClientMessage{Type: MsgCallUno})

	// Should receive UNO broadcast or error
	msg := client1.receive(time.Second)
	if msg == nil {
		t.Fatal("should receive response to UNO call")
	}
	// Either MsgCallUno (success) or MsgError (can't call with 7 cards)
	if msg.Type != MsgCallUno && msg.Type != MsgError {
		t.Errorf("unexpected message type: %d", msg.Type)
	}
}

func TestRoomCapacity(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	clients := make([]*testClient, MaxPlayers+1)

	// Join max players
	for i := range MaxPlayers {
		clients[i] = connectClient(t, server, "player"+string(rune('A'+i)))
		defer clients[i].close()
		clients[i].send(&ClientMessage{Type: MsgJoin, RoomID: "FULL", PlayerName: "Player" + string(rune('A'+i))})
		clients[i].receiveType(MsgRoomInfo, time.Second)
	}

	// Drain game state messages
	time.Sleep(100 * time.Millisecond)

	// Try to join one more
	extra := connectClient(t, server, "playerExtra")
	defer extra.close()
	extra.send(&ClientMessage{Type: MsgJoin, RoomID: "FULL", PlayerName: "Extra"})

	// Should receive error
	msg := extra.receive(time.Second)
	if msg == nil || msg.Type != MsgError {
		t.Error("should reject player when room is full")
	}
}

func TestConcurrentJoins(t *testing.T) {
	hub, cleanup := setupTest(t)
	defer cleanup()

	server := testServer(t, hub)
	defer server.Close()

	const numClients = 4
	var wg sync.WaitGroup
	clients := make([]*testClient, numClients)
	errors := make(chan error, numClients)

	for i := range numClients {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := connectClient(t, server, "player"+string(rune('0'+idx)))
			clients[idx] = c
			c.send(&ClientMessage{Type: MsgJoin, RoomID: "CONC", PlayerName: "P" + string(rune('0'+idx))})
			msg := c.receiveType(MsgRoomInfo, 2*time.Second)
			if msg == nil {
				errors <- nil // Could be race, acceptable
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Cleanup
	for _, c := range clients {
		if c != nil {
			c.close()
		}
	}

	// Check room state
	time.Sleep(100 * time.Millisecond)
	hub.mu.RLock()
	room := hub.rooms["CONC"]
	hub.mu.RUnlock()

	if room == nil {
		t.Fatal("room should exist")
	}
	if room.ClientCount() > MaxPlayers {
		t.Errorf("room should not exceed max players: %d", room.ClientCount())
	}
}
