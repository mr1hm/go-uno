//go:build js && wasm

package render

import (
	"encoding/json"
	"syscall/js"

	"github.com/mr1hm/go-uno/internal/game"
	"github.com/mr1hm/go-uno/internal/server"
)

// NetworkClient handles WebSocket communication in WASM
type NetworkClient struct {
	ws         js.Value
	connected  bool
	roomID     string
	playerID   string
	playerName string
	messages   chan server.ServerMessage
	onState    func(*server.ServerMessage)
}

// NewNetworkClient creates a new WebSocket client
func NewNetworkClient() *NetworkClient {
	return &NetworkClient{
		messages: make(chan server.ServerMessage, 32),
	}
}

// Connect establishes WebSocket connection
func (nc *NetworkClient) Connect(serverURL string) error {
	nc.ws = js.Global().Get("WebSocket").New(serverURL)

	nc.ws.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		nc.connected = true
		return nil
	}))

	nc.ws.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		nc.connected = false
		return nil
	}))

	nc.ws.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		data := args[0].Get("data").String()
		var msg server.ServerMessage
		if err := json.Unmarshal([]byte(data), &msg); err == nil {
			select {
			case nc.messages <- msg:
			default:
				// Channel full, drop message
			}
		}
		return nil
	}))

	return nil
}

// JoinRoom sends join request
func (nc *NetworkClient) JoinRoom(roomID, playerName string) {
	nc.playerName = playerName
	msg := server.ClientMessage{
		Type:       server.MsgJoin,
		RoomID:     roomID,
		PlayerName: playerName,
	}
	nc.send(msg)
}

// SendPlayCard sends play card action
func (nc *NetworkClient) SendPlayCard(cardIndex int, chosenColor game.Color) {
	msg := server.ClientMessage{
		Type:        server.MsgPlayCard,
		CardIndex:   cardIndex,
		ChosenColor: int(chosenColor),
	}
	nc.send(msg)
}

// SendDrawCard sends draw card action
func (nc *NetworkClient) SendDrawCard() {
	nc.send(server.ClientMessage{Type: server.MsgDrawCard})
}

// SendCallUno sends UNO call
func (nc *NetworkClient) SendCallUno() {
	nc.send(server.ClientMessage{Type: server.MsgCallUno})
}

// SendChallengeUno sends UNO challenge
func (nc *NetworkClient) SendChallengeUno(targetID string) {
	nc.send(server.ClientMessage{Type: server.MsgChallengeUno, TargetID: targetID})
}

// SendPass sends pass action
func (nc *NetworkClient) SendPass() {
	nc.send(server.ClientMessage{Type: server.MsgPass})
}

func (nc *NetworkClient) send(msg server.ClientMessage) {
	if !nc.connected {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	nc.ws.Call("send", string(data))
}

// Poll checks for new messages (non-blocking)
func (nc *NetworkClient) Poll() *server.ServerMessage {
	select {
	case msg := <-nc.messages:
		return &msg
	default:
		return nil
	}
}

// IsConnected returns connection status
func (nc *NetworkClient) IsConnected() bool {
	return nc.connected
}

// Close closes the connection
func (nc *NetworkClient) Close() {
	if nc.ws.Truthy() {
		nc.ws.Call("close")
	}
}
