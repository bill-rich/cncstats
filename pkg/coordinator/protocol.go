package coordinator

import "encoding/json"

const ProtocolVersion = 1

const (
	MsgHello      = "hello"
	MsgHelloOK    = "hello_ok"
	MsgDiscoverOK = "discover_ok"
	MsgHost       = "host"
	MsgHosted     = "hosted"
	MsgUnhost     = "unhost"
	MsgList       = "list"
	MsgGames      = "games"
	MsgJoin       = "join"
	MsgPeerInfo     = "peer_info"
	MsgHeartbeat    = "heartbeat"
	MsgPunchOutcome = "punch_outcome"
	MsgError        = "error"
	MsgBye          = "bye"
)

type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type Hello struct {
	Nick    string `json:"nick"`
	Version string `json:"version"`
}

type HelloOK struct {
	SessionToken string `json:"session_token"`
	STUNMagic    uint32 `json:"stun_magic"`
	UDPPort      int    `json:"udp_port"`
}

type DiscoverOK struct {
	PublicAddr string `json:"public_addr"`
}

type Host struct {
	Name           string `json:"name"`
	Map            string `json:"map"`
	MaxPlayers     int    `json:"max_players"`
	LocalAddr      string `json:"local_addr"`
	PublicAddr     string `json:"public_addr"`
	GamePublicAddr string `json:"game_public_addr,omitempty"`
}

type Hosted struct {
	GameID string `json:"game_id"`
}

type GameInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Map        string `json:"map"`
	HostNick   string `json:"host_nick"`
	Players    int    `json:"players"`
	MaxPlayers int    `json:"max_players"`
}

type Games struct {
	Games []GameInfo `json:"games"`
}

type Join struct {
	GameID         string `json:"game_id"`
	LocalAddr      string `json:"local_addr"`
	PublicAddr     string `json:"public_addr"`
	GamePublicAddr string `json:"game_public_addr,omitempty"`
}

type PeerInfo struct {
	Nick           string `json:"nick"`
	PublicAddr     string `json:"public_addr"`
	GamePublicAddr string `json:"game_public_addr,omitempty"`
	LocalAddr      string `json:"local_addr"`
	PunchInMS      int    `json:"punch_in_ms"`
	Role           string `json:"role"`
}

// PunchOutcome is fire-and-forget telemetry from a client after a hole
// punch attempt, so real-world punch success rates are visible server-side.
type PunchOutcome struct {
	OK      bool   `json:"ok"`
	LobbyOK bool   `json:"lobby_ok"`
	GameOK  bool   `json:"game_ok"`
	MS      int    `json:"ms"`
	Role    string `json:"role"`
}

type Error struct {
	Message string `json:"message"`
}

// STUNPurpose tags each STUN probe so the server can store two discovered
// addresses per session: one for the lobby socket (8086) and one for the
// in-game data socket (8088). Sent as the 21st byte of the STUN request,
// immediately after the 16-byte session token. Older clients that send only
// 20 bytes are treated as STUNPurposeLobby for backward compatibility.
const (
	STUNPurposeLobby byte = 0
	STUNPurposeGame  byte = 1
)

const (
	STUNRequestSize   = 4 + 16 + 1
	STUNResponseSize  = 4 + 4 + 2
	SessionTokenBytes = 16
)
