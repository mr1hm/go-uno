//go:build !js || !wasm

package render

import (
	"github.com/mr1hm/go-uno/internal/game"
	"github.com/mr1hm/go-uno/internal/server"
)

// NetworkClient stub for non-WASM builds
type NetworkClient struct{}

func NewNetworkClient() *NetworkClient {
	return &NetworkClient{}
}

func (nc *NetworkClient) Connect(serverURL string) error {
	return nil
}

func (nc *NetworkClient) JoinRoom(roomID, playerName string) {}

func (nc *NetworkClient) SendPlayCard(cardIndex int, chosenColor game.Color) {}

func (nc *NetworkClient) SendDrawCard() {}

func (nc *NetworkClient) SendCallUno() {}

func (nc *NetworkClient) SendChallengeUno(targetID string) {}

func (nc *NetworkClient) SendPass() {}

func (nc *NetworkClient) Poll() *server.ServerMessage {
	return nil
}

func (nc *NetworkClient) IsConnected() bool {
	return false
}

func (nc *NetworkClient) Close() {}
