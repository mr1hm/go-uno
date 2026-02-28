// Discord client ID - set this from env or config
const DISCORD_CLIENT_ID = window.DISCORD_CLIENT_ID || 'YOUR_CLIENT_ID';

let discordSdk = null;
let auth = null;
let DiscordSDK = null;

// Exported state for Go/WASM to read
window.discordState = {
    ready: false,
    instanceId: '',
    channelId: '',
    guildId: '',
    userId: '',
    username: '',
    participants: []
};

async function initDiscord() {
    try {
        // Load Discord SDK dynamically
        const module = await import('https://cdn.jsdelivr.net/npm/@discord/embedded-app-sdk/+esm');
        DiscordSDK = module.DiscordSDK;

        // Initialize Discord SDK
        discordSdk = new DiscordSDK(DISCORD_CLIENT_ID);
        await discordSdk.ready();

        // Authorize with Discord
        const { code } = await discordSdk.commands.authorize({
            client_id: DISCORD_CLIENT_ID,
            response_type: 'code',
            state: '',
            prompt: 'none',
            scope: ['identify', 'guilds']
        });

        // Exchange code for token (call our server)
        const response = await fetch('/api/token', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code })
        });

        if (!response.ok) {
            throw new Error('Failed to get token');
        }

        const { access_token } = await response.json();

        // Authenticate with Discord
        auth = await discordSdk.commands.authenticate({ access_token });

        // Get instance info
        window.discordState.instanceId = discordSdk.instanceId;
        window.discordState.channelId = discordSdk.channelId;
        window.discordState.guildId = discordSdk.guildId;
        window.discordState.userId = auth.user.id;
        window.discordState.username = auth.user.username;
        window.discordState.ready = true;

        console.log('Discord SDK ready:', window.discordState);

        // Subscribe to participant updates
        discordSdk.subscribe('ACTIVITY_INSTANCE_PARTICIPANTS_UPDATE', (data) => {
            window.discordState.participants = data.participants.map(p => ({
                id: p.id,
                username: p.username
            }));
            // Notify Go if callback is set
            if (window.onParticipantsUpdate) {
                window.onParticipantsUpdate(window.discordState.participants);
            }
        });

        // Dispatch ready event
        window.dispatchEvent(new Event('discord-ready'));

    } catch (error) {
        console.error('Discord SDK init failed:', error);
        // Still dispatch ready event so game can run (for local testing)
        window.discordState.ready = false;
        window.dispatchEvent(new Event('discord-ready'));
    }
}

// Start initialization
initDiscord();

// Export for debugging
window.discordSdk = discordSdk;
