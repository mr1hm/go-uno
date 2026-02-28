//go:build js && wasm

package render

import "syscall/js"

// DiscordState holds the current Discord Activity state
type DiscordState struct {
	Ready      bool
	InstanceID string
	ChannelID  string
	GuildID    string
	UserID     string
	Username   string
}

// GetDiscordState reads the current Discord state from JavaScript
func GetDiscordState() DiscordState {
	state := js.Global().Get("discordState")
	if !state.Truthy() {
		return DiscordState{}
	}

	return DiscordState{
		Ready:      state.Get("ready").Bool(),
		InstanceID: state.Get("instanceId").String(),
		ChannelID:  state.Get("channelId").String(),
		GuildID:    state.Get("guildId").String(),
		UserID:     state.Get("userId").String(),
		Username:   state.Get("username").String(),
	}
}

// IsDiscordActivity returns true if running inside Discord
func IsDiscordActivity() bool {
	state := GetDiscordState()
	return state.Ready && state.InstanceID != ""
}

// GetHost returns the current host for WebSocket connection
func getHostWASM() string {
	location := js.Global().Get("window").Get("location")
	if !location.Truthy() {
		return "localhost:3000"
	}
	return location.Get("host").String()
}
