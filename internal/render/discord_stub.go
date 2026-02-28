//go:build !js || !wasm

package render

// DiscordState holds the current Discord Activity state
type DiscordState struct {
	Ready      bool
	InstanceID string
	ChannelID  string
	GuildID    string
	UserID     string
	Username   string
}

// GetDiscordState returns empty state for non-WASM builds
func GetDiscordState() DiscordState {
	return DiscordState{}
}

// IsDiscordActivity returns false for non-WASM builds
func IsDiscordActivity() bool {
	return false
}

// getHostWASM returns localhost for non-WASM builds
func getHostWASM() string {
	return "localhost:3000"
}
