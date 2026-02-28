package server

import "github.com/mr1hm/go-uno/internal/game"

// Message types (single byte for efficiency)
const (
	MsgJoin byte = iota
	MsgLeave
	MsgGameState
	MsgPlayCard
	MsgDrawCard
	MsgCallUno
	MsgChallengeUno
	MsgPass
	MsgError
	MsgPlayerJoined
	MsgPlayerLeft
	MsgGameOver
	MsgRoomInfo
)

// ClientMessage is sent from client to server
type ClientMessage struct {
	Type        byte   `json:"t"`
	RoomID      string `json:"r,omitempty"`
	PlayerName  string `json:"n,omitempty"`
	CardIndex   int    `json:"c,omitempty"`
	ChosenColor int    `json:"col,omitempty"` // For wild cards
	TargetID    string `json:"tid,omitempty"` // For UNO challenge
}

// ServerMessage is sent from server to client
type ServerMessage struct {
	Type          byte          `json:"t"`
	Error         string        `json:"e,omitempty"`
	RoomID        string        `json:"r,omitempty"`
	PlayerID      string        `json:"pid,omitempty"`
	PlayerName    string        `json:"n,omitempty"`
	Players       []PlayerInfo  `json:"p,omitempty"`
	CurrentPlayer int           `json:"cp,omitempty"`
	Direction     int           `json:"d,omitempty"`
	TopCard       *CardInfo     `json:"tc,omitempty"`
	ChosenColor   int           `json:"col,omitempty"`
	Hand          []CardInfo    `json:"h,omitempty"`
	Winner        string        `json:"w,omitempty"`
	Action        string        `json:"a,omitempty"` // For action announcements
}

// PlayerInfo is minimal player data sent to clients
type PlayerInfo struct {
	ID           string `json:"id"`
	Name         string `json:"n"`
	CardCount    int    `json:"c"`
	HasCalledUno bool   `json:"u,omitempty"`
}

// CardInfo is minimal card data
type CardInfo struct {
	Color int `json:"c"`
	Value int `json:"v"`
}

// ToCardInfo converts game.Card to CardInfo
func ToCardInfo(c game.Card) CardInfo {
	return CardInfo{Color: int(c.Color), Value: int(c.Value)}
}

// ToCard converts CardInfo back to game.Card
func (ci CardInfo) ToCard() game.Card {
	return game.Card{Color: game.Color(ci.Color), Value: game.Value(ci.Value)}
}

// CardsToInfo converts a slice of cards
func CardsToInfo(cards []game.Card) []CardInfo {
	info := make([]CardInfo, len(cards))
	for i, c := range cards {
		info[i] = ToCardInfo(c)
	}
	return info
}
