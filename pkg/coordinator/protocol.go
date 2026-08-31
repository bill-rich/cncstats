package coordinator

import "encoding/json"

const ProtocolVersion = 1

const (
	MsgHello        = "hello"
	MsgHelloOK      = "hello_ok"
	MsgDiscoverOK   = "discover_ok"
	MsgHost         = "host"
	MsgHosted       = "hosted"
	MsgUnhost       = "unhost"
	MsgList         = "list"
	MsgGames        = "games"
	MsgJoin         = "join"
	MsgPeerInfo     = "peer_info"
	MsgHeartbeat    = "heartbeat"
	MsgPunchOutcome = "punch_outcome"
	MsgError        = "error"
	MsgBye          = "bye"

	// Observing in-progress games. The host reports game_started so its
	// listing flips to in-progress; a viewer sends observe; the server
	// mints a relay token, notifies the host (observer_request) and the
	// viewer (observe_ok); then BOTH sides dial fresh TCP connections whose
	// first line is relay_attach, and the server splices the two
	// connections into one byte pipe carrying the observer stream.
	MsgGameStarted     = "game_started"
	MsgObserve         = "observe"
	MsgObserveOK       = "observe_ok"
	MsgObserverRequest = "observer_request"
	MsgRelayAttach     = "relay_attach"

	// UDP relay fallback: sent to BOTH members of a punch pair when either
	// side reports a failed punch, telling each client to route that pair's
	// lobby and game traffic through the coordinator's UDP relay.
	MsgRelayGrant = "relay_grant"
)

type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type Hello struct {
	Nick    string `json:"nick"`
	Version string `json:"version"`
	// Nonzero: this client understands the UDP relay fallback (RelayPurpose*
	// frames and relay_grant). The server only mints relay ids and grants
	// relays between clients that both advertised support.
	Relay int `json:"relay,omitempty"`
}

type HelloOK struct {
	SessionToken string `json:"session_token"`
	STUNMagic    uint32 `json:"stun_magic"`
	UDPPort      int    `json:"udp_port"`
	// The client's own relay id; 0/absent when relaying is unavailable.
	// Numeric because the VC6 client's JSON parser only reads numeric and
	// string fields.
	RelayID uint32 `json:"relay_id,omitempty"`
	// Second STUN port for NAT self-classification: a client that observes
	// DIFFERENT external ports from probes to the two ports has an
	// endpoint-dependent (symmetric) mapping and is warned before hosting.
	// 0/absent on servers without the second listener.
	UDPPort2 int `json:"udp_port2,omitempty"`
}

type DiscoverOK struct {
	PublicAddr string `json:"public_addr"`
}

type Host struct {
	Name           string `json:"name"`
	Map            string `json:"map"`
	MaxPlayers     int    `json:"max_players"`
	LocalAddr      string `json:"local_addr"`
	GameLocalAddr  string `json:"game_local_addr,omitempty"`
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
	// 1 once the host reported game_started. An int (not bool) because the
	// VC6 client's hand-rolled JSON parser only reads numeric fields.
	InProgress int `json:"in_progress"`
	// 1 once any guest's punch against this host failed or relayed: the
	// host has a restrictive connection, and the lobby browser marks the
	// game so players can prefer a different host.
	RestrictedHost int `json:"restricted_host,omitempty"`
}

type Games struct {
	Games []GameInfo `json:"games"`
}

type Join struct {
	GameID         string `json:"game_id"`
	LocalAddr      string `json:"local_addr"`
	GameLocalAddr  string `json:"game_local_addr,omitempty"`
	PublicAddr     string `json:"public_addr"`
	GamePublicAddr string `json:"game_public_addr,omitempty"`
}

// PeerInfo's Role is the RECEIVER's role relative to the described peer:
// "host"/"guest" for the synchronized host<->guest punch pair, or "peer"
// for a guest<->guest mesh notification (no synchronized punch; each side
// just opens NAT mappings toward the other from its lobby and game sockets).
type PeerInfo struct {
	Nick           string `json:"nick"`
	PublicAddr     string `json:"public_addr"`
	GamePublicAddr string `json:"game_public_addr,omitempty"`
	LocalAddr      string `json:"local_addr"`
	// The peer's in-game (8088) bind address on that same interface. A
	// same-LAN pair needs a local candidate on the game channel too, not
	// just the lobby, or it can only reach 8088 by hairpinning off the
	// shared public address. Absent from older clients.
	GameLocalAddr string `json:"game_local_addr,omitempty"`
	PunchInMS     int    `json:"punch_in_ms"`
	Role          string `json:"role"`
	// The described peer's relay id, present only when both this client and
	// the peer advertised relay support. The receiver registers it in its
	// relay registry so a failed punch can flip the pair to the relay.
	RelayID uint32 `json:"relay_id,omitempty"`
}

// PunchOutcome is fire-and-forget telemetry from a client after a hole
// punch attempt, so real-world punch success rates are visible server-side.
type PunchOutcome struct {
	OK      bool   `json:"ok"`
	LobbyOK bool   `json:"lobby_ok"`
	GameOK  bool   `json:"game_ok"`
	MS      int    `json:"ms"`
	Role    string `json:"role"`
	// Relayed means the client resolved the failed punch by flipping the
	// pair to the relay (so the join is proceeding, not failing): the game
	// bookkeeping rollback must not run, and the peer named by PeerRelayID
	// gets a relay_grant so both sides converge.
	Relayed     bool   `json:"relayed,omitempty"`
	PeerRelayID uint32 `json:"peer_relay_id,omitempty"`
}

// RelayGrant tells a client to route everything for the peer with this
// relay id through the coordinator's UDP relay.
type RelayGrant struct {
	PeerRelayID uint32 `json:"peer_relay_id"`
	PeerNick    string `json:"peer_nick,omitempty"`
}

type Observe struct {
	GameID string `json:"game_id"`
}

type ObserveOK struct {
	Token string `json:"token"`
}

type ObserverRequest struct {
	Token string `json:"token"`
}

type RelayAttach struct {
	Token string `json:"token"`
	Role  string `json:"role"` // "host" or "viewer"
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
	// UDP relay fallback frames share the STUN socket and header layout up
	// through the purpose byte (magic + token + purpose):
	//  client->server RelayData:    magic(4) token(16) purpose=2 channel(1) destRelayID(4 BE) payload
	//    destRelayID 0 with no payload is a keepalive that (re)registers the
	//    sender's return address for that channel.
	//  server->client RelayDeliver: magic(4) purpose=3 channel(1) srcRelayID(4 BE) payload
	RelayPurposeData    byte = 2
	RelayPurposeDeliver byte = 3
)

const (
	STUNRequestSize   = 4 + 16 + 1
	STUNResponseSize  = 4 + 4 + 2
	SessionTokenBytes = 16

	RelayDataHeaderSize    = 4 + SessionTokenBytes + 1 + 1 + 4
	RelayDeliverHeaderSize = 4 + 1 + 1 + 4
	// Max relayed payload: the client's max UDP payload (1100) plus slack.
	RelayMaxPayload = 1200

	RelayChannelLobby byte = 0
	RelayChannelGame  byte = 1
)
