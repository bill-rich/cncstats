// Package coordinator is the online matchmaking coordinator for the Zulu
// game client: a TCP signaling channel (newline-delimited JSON) plus a tiny
// binary STUN-style UDP responder used for public-address discovery and NAT
// hole punching. The reference client lives in the game repo at
// Core/GameEngine/Source/GameNetwork/OnlineCoordinatorAPI.cpp; keep
// protocol.go in sync with tools/coordinator/protocol.go there.
package coordinator

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// logger tags every coordinator line with component=coordinator so a day's
// matchmaking traffic can be pulled out of the shared application log with
// one grep (the persistent copy lives under LOGS_DIR/server; see
// pkg/serverlog).
var logger = log.WithField("component", "coordinator")

// TCPAddr and UDPAddr are the listen addresses, overridable via environment
// variables (matching the STATS_DIR/MAPS_DIR convention elsewhere in this
// repo). The game client defaults to TCP 27500 / UDP 27501.
var (
	TCPAddr  = ":27500"
	UDPAddr  = ":27501"
	UDP2Addr = ":27503"
)

func init() {
	if v := os.Getenv("COORD_TCP_ADDR"); v != "" {
		TCPAddr = v
	}
	if v := os.Getenv("COORD_UDP_ADDR"); v != "" {
		UDPAddr = v
	}
	if v := os.Getenv("COORD_UDP2_ADDR"); v != "" {
		UDP2Addr = v
	}
}

const (
	DefaultSTUNMagic = uint32(0x5A554C55) // "ZULU"
	PunchDelayMS     = 750
	SessionTTL       = 5 * time.Minute
	ReapInterval     = 30 * time.Second
	WriteTimeout     = 5 * time.Second
)

type Session struct {
	Token          string
	Conn           net.Conn
	Nick           string
	Version        string
	RemoteAddr     string
	PublicAddr     string // lobby (8086) public addr, discovered via STUN purpose=0
	GamePublicAddr string // in-game (8088) public addr, discovered via STUN purpose=1
	LocalAddr      string
	HostingGame    string
	JoiningGame    string // game id of the last join attempt (player-count bookkeeping)
	RelayID        uint32 // nonzero when the client advertised relay support
	Started        time.Time
	LastSeen       time.Time
	writeMu        sync.Mutex
}

// relayPeer is the UDP relay's routing state for one client. Deliberately
// decoupled from Session: joiners tear their signaling TCP down at game
// start, but their relayed in-game traffic must keep flowing, so routing is
// keyed by the session token carried in every RelayData frame and refreshed
// by that traffic (and by STUN keepalive probes, which arrive from the same
// sockets and double as return-address updates).
type relayPeer struct {
	relayID   uint32
	token     string
	nick      string
	lobbyAddr *net.UDPAddr // return addr for channel 0
	gameAddr  *net.UDPAddr // return addr for channel 1
	lastSeen  time.Time
	// Cheap per-second rate limit.
	rateSec   int64
	ratePkts  int
	rateBytes int
}

// relayPairState marks two relay ids as introduced to each other via
// peer_info; the relay only ever forwards between introduced pairs.
type relayPairState struct {
	lastSeen time.Time
	logged   bool // first forwarded frame for this pair logged
}

const (
	// A relayed match refreshes these continuously; anything idle this long
	// is dead.
	relayPeerTTL = 15 * time.Minute
	relayPairTTL = 15 * time.Minute
	// Per-sender caps, far above the lockstep traffic of one 8-player game.
	relayMaxPktsPerSec  = 800
	relayMaxBytesPerSec = 400 * 1024
)

func relayPairKey(a, b uint32) uint64 {
	if a > b {
		a, b = b, a
	}
	return uint64(a)<<32 | uint64(b)
}

func (sess *Session) send(msgType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	env := Envelope{Type: msgType, Data: raw}
	line, err := json.Marshal(env)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	sess.writeMu.Lock()
	defer sess.writeMu.Unlock()
	sess.Conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	_, err = sess.Conn.Write(line)
	sess.Conn.SetWriteDeadline(time.Time{})
	return err
}

type gameState struct {
	info     GameInfo
	hostSess *Session
	// Guests that have been matched into this game, in join order. Used to
	// orchestrate guest<->guest hole punching: the coordinator only ever
	// punched host<->guest pairs, so two guests behind restricted NATs had
	// no mutual mappings and their in-game keepalive/file-transfer legs
	// stalled until NAT state self-healed. On every join, each existing
	// guest and the new guest are sent a peer_info with role "peer" so both
	// sides open mappings toward each other while still in the lobby.
	guests  []*Session
	created time.Time
}

// relayState pairs the host's and viewer's relay_attach connections for one
// observer session. Created when the observe request is accepted; each side
// fills in its slot when its attach connection arrives; spliced when both
// are present. Unpaired conns/tokens are reaped after RelayAttachTTL.
type relayState struct {
	created time.Time
	host    net.Conn
	hostR   *bufio.Reader
	viewer  net.Conn
	viewerR *bufio.Reader
}

const RelayAttachTTL = 60 * time.Second

type Server struct {
	Magic   uint32
	UDPPort int
	// Second STUN listener for NAT self-classification; 0 when disabled.
	UDPPort2 int

	mu       sync.Mutex
	sessions map[string]*Session
	games    map[string]*gameState
	relays   map[string]*relayState
	// Lifetime punch telemetry counters (see MsgPunchOutcome).
	punchOK   int
	punchFail int

	// UDP relay fallback state.
	nextRelayID  uint32
	relayPeers   map[uint32]*relayPeer
	relayByToken map[string]*relayPeer
	relayPairs   map[uint64]*relayPairState
	// Lifetime relay telemetry counters.
	punchRelayed    int
	relayForwarded  int64 // packets forwarded
	relayBytes      int64 // payload bytes forwarded
	relayDropped    int64 // frames dropped (unknown dest, unpaired, no addr, rate limit)
	relayGrantsSent int
}

func NewServer() *Server {
	return &Server{
		Magic:        DefaultSTUNMagic,
		sessions:     make(map[string]*Session),
		games:        make(map[string]*gameState),
		relays:       make(map[string]*relayState),
		nextRelayID:  1,
		relayPeers:   make(map[uint32]*relayPeer),
		relayByToken: make(map[string]*relayPeer),
		relayPairs:   make(map[uint64]*relayPairState),
	}
}

func (s *Server) Run(tcpAddr, udpAddr string) error {
	return s.RunWithAltSTUN(tcpAddr, udpAddr, UDP2Addr)
}

// RunWithAltSTUN also opens a second STUN-only UDP listener when udp2Addr
// is nonempty. Clients probe both from one socket; differing observed
// ports reveal an endpoint-dependent (symmetric) NAT, which the client
// uses to warn would-be hosts. The second port serves STUN replies only
// (relay frames on it are ignored).
func (s *Server) RunWithAltSTUN(tcpAddr, udpAddr, udp2Addr string) error {
	tcpL, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("listen tcp: %w", err)
	}
	defer tcpL.Close()
	logger.Printf("TCP signaling listening on %s", tcpL.Addr())

	udpResAddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("resolve udp: %w", err)
	}
	udpL, err := net.ListenUDP("udp", udpResAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	defer udpL.Close()
	logger.Printf("UDP STUN listening on %s", udpL.LocalAddr())
	s.UDPPort = udpL.LocalAddr().(*net.UDPAddr).Port

	if udp2Addr != "" {
		udp2ResAddr, err := net.ResolveUDPAddr("udp", udp2Addr)
		if err != nil {
			return fmt.Errorf("resolve udp2: %w", err)
		}
		udp2L, err := net.ListenUDP("udp", udp2ResAddr)
		if err != nil {
			return fmt.Errorf("listen udp2: %w", err)
		}
		defer udp2L.Close()
		logger.Printf("UDP STUN (alt, NAT check) listening on %s", udp2L.LocalAddr())
		s.UDPPort2 = udp2L.LocalAddr().(*net.UDPAddr).Port
		go s.handleUDPOn(udp2L, true)
	}

	go s.handleUDP(udpL)
	go s.reapLoop()
	return s.acceptTCP(tcpL)
}

func (s *Server) acceptTCP(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			logger.Printf("accept: %v", err)
			continue
		}
		go s.handleTCPConn(conn)
	}
}

func newToken() (string, error) {
	b := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Server) handleTCPConn(conn net.Conn) {
	handedOff := false
	defer func() {
		if !handedOff {
			conn.Close()
		}
	}()
	r := bufio.NewReaderSize(conn, 8192)

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	line, err := r.ReadBytes('\n')
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return
	}
	var env Envelope
	if err := json.Unmarshal(line, &env); err != nil {
		return
	}
	// A relay_attach connection is not a session: after this one line the
	// socket becomes a raw byte pipe for the observer stream.
	if env.Type == MsgRelayAttach {
		var ra RelayAttach
		if err := json.Unmarshal(env.Data, &ra); err != nil {
			return
		}
		handedOff = s.handleRelayAttach(conn, r, &ra)
		return
	}
	if env.Type != MsgHello {
		return
	}
	var hello Hello
	if err := json.Unmarshal(env.Data, &hello); err != nil {
		return
	}

	token, err := newToken()
	if err != nil {
		return
	}
	sess := &Session{
		Token:      token,
		Conn:       conn,
		Nick:       sanitizeNick(hello.Nick),
		Version:    hello.Version,
		RemoteAddr: conn.RemoteAddr().String(),
		Started:    time.Now(),
		LastSeen:   time.Now(),
	}

	s.mu.Lock()
	s.sessions[token] = sess
	if hello.Relay > 0 {
		sess.RelayID = s.nextRelayID
		s.nextRelayID++
		if s.nextRelayID == 0 { // wrapped; 0 is "no relay"
			s.nextRelayID = 1
		}
		rp := &relayPeer{
			relayID:  sess.RelayID,
			token:    token,
			nick:     sess.Nick,
			lastSeen: time.Now(),
		}
		s.relayPeers[sess.RelayID] = rp
		s.relayByToken[token] = rp
	}
	s.mu.Unlock()
	defer s.dropSession(sess)

	if err := sess.send(MsgHelloOK, HelloOK{
		SessionToken: token,
		STUNMagic:    s.Magic,
		UDPPort:      s.UDPPort,
		RelayID:      sess.RelayID,
		UDPPort2:     s.UDPPort2,
	}); err != nil {
		return
	}
	logger.Printf("session %s nick=%q version=%s relay=%d from %s", token[:8], sess.Nick, sess.Version, sess.RelayID, sess.RemoteAddr)

	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				logger.Printf("session %s read: %v", token[:8], err)
			}
			return
		}
		s.mu.Lock()
		sess.LastSeen = time.Now()
		s.mu.Unlock()
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			logger.Printf("session %s nick=%q rejected: bad envelope (%v)", token[:8], sess.Nick, err)
			sess.send(MsgError, Error{Message: "bad envelope"})
			continue
		}
		if err := s.handleMessage(sess, &env); err != nil {
			if err == io.EOF {
				return
			}
			// Every rejection the client is shown is recorded here: this is
			// the server's only record of why a player's host/join attempt
			// failed, and the client-side ReleaseLog it uploads on failure
			// is matched against it by nick + time.
			logger.Printf("session %s nick=%q version=%s from %s: %s REJECTED: %v",
				token[:8], sess.Nick, sess.Version, sess.RemoteAddr, env.Type, err)
			sess.send(MsgError, Error{Message: err.Error()})
		}
	}
}

// handleRelayAttach wires one side of an observer relay. Returns true when
// the connection has been adopted (caller must not close it).
func (s *Server) handleRelayAttach(conn net.Conn, r *bufio.Reader, ra *RelayAttach) bool {
	s.mu.Lock()
	slot, ok := s.relays[ra.Token]
	if !ok {
		s.mu.Unlock()
		logger.Printf("relay_attach: unknown token from %s", conn.RemoteAddr())
		return false
	}
	switch ra.Role {
	case "host":
		if slot.host != nil {
			s.mu.Unlock()
			return false
		}
		slot.host, slot.hostR = conn, r
	case "viewer":
		if slot.viewer != nil {
			s.mu.Unlock()
			return false
		}
		slot.viewer, slot.viewerR = conn, r
	default:
		s.mu.Unlock()
		return false
	}
	ready := slot.host != nil && slot.viewer != nil
	if ready {
		delete(s.relays, ra.Token)
	}
	s.mu.Unlock()

	logger.Printf("relay_attach: token=%s role=%s from %s (paired=%v)",
		ra.Token[:8], ra.Role, conn.RemoteAddr(), ready)
	if ready {
		go spliceRelay(slot)
	}
	return true
}

// spliceRelay pipes bytes between the paired host and viewer connections
// until either side closes. The observer stream is host->viewer, but both
// directions are piped so future control traffic would survive too. The
// bufio readers are used for reads so no bytes buffered behind the attach
// line are lost.
func spliceRelay(slot *relayState) {
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(slot.viewer, slot.hostR)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(slot.host, slot.viewerR)
		done <- struct{}{}
	}()
	<-done
	slot.host.Close()
	slot.viewer.Close()
	<-done
	logger.Printf("relay closed (host=%s viewer=%s)", slot.host.RemoteAddr(), slot.viewer.RemoteAddr())
}

func (s *Server) handleMessage(sess *Session, env *Envelope) error {
	switch env.Type {
	case MsgHeartbeat:
		return nil
	case MsgGameStarted:
		s.mu.Lock()
		if sess.HostingGame != "" {
			if g, ok := s.games[sess.HostingGame]; ok {
				g.info.InProgress = 1
			}
		}
		s.mu.Unlock()
		logger.Printf("game started: session=%s game=%s", sess.Token[:8], sess.HostingGame)
		return nil
	case MsgObserve:
		var m Observe
		if err := json.Unmarshal(env.Data, &m); err != nil {
			return err
		}
		return s.handleObserve(sess, &m)
	case MsgPunchOutcome:
		var m PunchOutcome
		if err := json.Unmarshal(env.Data, &m); err != nil {
			return err
		}
		// A relayed outcome means the join is proceeding over the relay:
		// count it separately and leave the game bookkeeping alone.
		var grantTo []*Session
		s.mu.Lock()
		// Any guest that could not punch this host marks the game listing:
		// the lobby browser shows "restricted host" so players can prefer a
		// different host, and the host's own client can surface the hint.
		if m.Role == "guest" && (!m.OK || m.Relayed) && sess.JoiningGame != "" {
			if g, ok := s.games[sess.JoiningGame]; ok && g.info.RestrictedHost == 0 {
				g.info.RestrictedHost = 1
				logger.Printf("game %s marked restricted_host (guest %q ok=%v relayed=%v)",
					sess.JoiningGame, sess.Nick, m.OK, m.Relayed)
			}
		}
		if m.OK {
			s.punchOK++
		} else if m.Relayed {
			s.punchRelayed++
		} else {
			s.punchFail++
			// Roll back the optimistic player count from handleJoin so a
			// failed punch (and its retry) doesn't inflate the listing, and
			// drop the guest from the mesh roster so later joiners are not
			// told to punch a peer that never made it into the game.
			if m.Role == "guest" && sess.JoiningGame != "" {
				if g, ok := s.games[sess.JoiningGame]; ok {
					if g.info.Players > 1 {
						g.info.Players--
					}
					for i, gg := range g.guests {
						if gg == sess {
							g.guests = append(g.guests[:i], g.guests[i+1:]...)
							break
						}
					}
				}
			}
		}
		if m.Role == "guest" && !m.Relayed {
			sess.JoiningGame = ""
		}
		// Failed or relayed punch with a relay-capable peer: grant the relay
		// to BOTH sides (idempotent client-side) so they converge on it. The
		// pair must have been introduced via peer_info.
		if (!m.OK || m.Relayed) && m.PeerRelayID != 0 && sess.RelayID != 0 {
			if _, paired := s.relayPairs[relayPairKey(sess.RelayID, m.PeerRelayID)]; paired {
				if peerRP, ok := s.relayPeers[m.PeerRelayID]; ok {
					if peerSess, ok := s.sessions[peerRP.token]; ok {
						grantTo = []*Session{sess, peerSess}
						s.relayGrantsSent += 2
					}
				}
			}
		}
		s.mu.Unlock()
		if grantTo != nil {
			// Sends run unlocked (they can block on a slow client).
			grantTo[0].send(MsgRelayGrant, RelayGrant{PeerRelayID: m.PeerRelayID, PeerNick: grantTo[1].Nick})
			grantTo[1].send(MsgRelayGrant, RelayGrant{PeerRelayID: sess.RelayID, PeerNick: sess.Nick})
			logger.Printf("relay grant: %s(id %d) <-> %s(id %d)",
				grantTo[0].Nick, sess.RelayID, grantTo[1].Nick, m.PeerRelayID)
		}
		logger.Printf("punch outcome session=%s nick=%q role=%s ok=%v lobby=%v game=%v relayed=%v ms=%d",
			sess.Token[:8], sess.Nick, m.Role, m.OK, m.LobbyOK, m.GameOK, m.Relayed, m.MS)
		return nil
	case MsgHost:
		var m Host
		if err := json.Unmarshal(env.Data, &m); err != nil {
			return err
		}
		return s.handleHost(sess, &m)
	case MsgUnhost:
		return s.handleUnhost(sess)
	case MsgList:
		return s.handleList(sess)
	case MsgJoin:
		var m Join
		if err := json.Unmarshal(env.Data, &m); err != nil {
			return err
		}
		return s.handleJoin(sess, &m)
	case MsgBye:
		return io.EOF
	default:
		return fmt.Errorf("unknown message type %q", env.Type)
	}
}

func (s *Server) handleHost(sess *Session, m *Host) error {
	if m.PublicAddr == "" {
		return fmt.Errorf("public_addr required (do STUN discovery first)")
	}
	if m.GamePublicAddr == "" {
		return fmt.Errorf("game_public_addr required (do STUN discovery on game port first)")
	}
	s.mu.Lock()
	if sess.HostingGame != "" {
		gid := sess.HostingGame
		s.mu.Unlock()
		return fmt.Errorf("already hosting game %s", gid)
	}
	gameID, err := newToken()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	gameID = gameID[:12]
	sess.LocalAddr = m.LocalAddr
	sess.PublicAddr = m.PublicAddr
	sess.GamePublicAddr = m.GamePublicAddr
	sess.HostingGame = gameID
	s.games[gameID] = &gameState{
		info: GameInfo{
			ID:         gameID,
			Name:       sanitizeName(m.Name),
			Map:        m.Map,
			HostNick:   sess.Nick,
			Players:    1,
			MaxPlayers: clampMaxPlayers(m.MaxPlayers),
		},
		hostSess: sess,
		created:  time.Now(),
	}
	s.mu.Unlock()
	logger.Printf("session %s hosting game %s name=%q", sess.Token[:8], gameID, m.Name)
	return sess.send(MsgHosted, Hosted{GameID: gameID})
}

func (s *Server) handleUnhost(sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess.HostingGame == "" {
		return nil
	}
	delete(s.games, sess.HostingGame)
	sess.HostingGame = ""
	return nil
}

func (s *Server) handleList(sess *Session) error {
	s.mu.Lock()
	out := Games{Games: make([]GameInfo, 0, len(s.games))}
	for _, g := range s.games {
		out.Games = append(out.Games, g.info)
	}
	s.mu.Unlock()
	return sess.send(MsgGames, out)
}

// handleObserve accepts a viewer's request to watch an in-progress game:
// mint a relay token, tell the host to attach its end, and hand the token
// back to the viewer so both relay connections can meet at spliceRelay.
func (s *Server) handleObserve(sess *Session, m *Observe) error {
	s.mu.Lock()
	g, ok := s.games[m.GameID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("game %s not found", m.GameID)
	}
	if g.info.InProgress == 0 {
		s.mu.Unlock()
		return fmt.Errorf("game has not started yet; join it instead")
	}
	host := g.hostSess
	if host == sess {
		s.mu.Unlock()
		return fmt.Errorf("cannot observe your own game")
	}
	if sess.Version != host.Version {
		s.mu.Unlock()
		return fmt.Errorf("game version mismatch: host is running %s, you are running %s",
			host.Version, sess.Version)
	}
	token, err := newToken()
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.relays[token] = &relayState{created: time.Now()}
	s.mu.Unlock()

	if err := host.send(MsgObserverRequest, ObserverRequest{Token: token}); err != nil {
		s.mu.Lock()
		delete(s.relays, token)
		s.mu.Unlock()
		return fmt.Errorf("host unreachable: %w", err)
	}
	logger.Printf("observe: viewer=%s game=%s token=%s", sess.Nick, m.GameID, token[:8])
	return sess.send(MsgObserveOK, ObserveOK{Token: token})
}

func (s *Server) handleJoin(sess *Session, m *Join) error {
	if m.PublicAddr == "" {
		return fmt.Errorf("public_addr required (do STUN discovery first)")
	}
	if m.GamePublicAddr == "" {
		return fmt.Errorf("game_public_addr required (do STUN discovery on game port first)")
	}
	s.mu.Lock()
	g, ok := s.games[m.GameID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("game %s not found", m.GameID)
	}
	if g.info.InProgress != 0 {
		s.mu.Unlock()
		return fmt.Errorf("game already started (observe it instead)")
	}
	host := g.hostSess
	if host == sess {
		s.mu.Unlock()
		return fmt.Errorf("cannot join your own game")
	}
	if sess.Version != host.Version {
		s.mu.Unlock()
		return fmt.Errorf("game version mismatch: host is running %s, you are running %s",
			host.Version, sess.Version)
	}
	sess.LocalAddr = m.LocalAddr
	sess.PublicAddr = m.PublicAddr
	sess.GamePublicAddr = m.GamePublicAddr
	// Approximate the listed player count (joiner leaves are not tracked,
	// but "2/4 filling up" beats a listing stuck at 1). A failed punch
	// reported via punch_outcome rolls this back so retries don't inflate
	// the listing.
	if g.info.Players < g.info.MaxPlayers {
		g.info.Players++
	}
	sess.JoiningGame = m.GameID
	// Snapshot both peers' fields while holding the lock; sends below run
	// unlocked and must not touch shared Session state.
	// Relay ids ride along only when BOTH sides support relaying; recording
	// the pair is what later authorizes grants and forwarding for it.
	pairRelayID := func(a, b *Session) (uint32, uint32) {
		if a.RelayID == 0 || b.RelayID == 0 {
			return 0, 0
		}
		s.relayPairs[relayPairKey(a.RelayID, b.RelayID)] = &relayPairState{lastSeen: time.Now()}
		return a.RelayID, b.RelayID
	}
	guestRelayID, hostRelayID := pairRelayID(sess, host)
	guestInfo := PeerInfo{
		Nick:           sess.Nick,
		PublicAddr:     sess.PublicAddr,
		GamePublicAddr: sess.GamePublicAddr,
		LocalAddr:      sess.LocalAddr,
		PunchInMS:      PunchDelayMS,
		Role:           "host",
		RelayID:        guestRelayID,
	}
	hostInfo := PeerInfo{
		Nick:           host.Nick,
		PublicAddr:     host.PublicAddr,
		GamePublicAddr: host.GamePublicAddr,
		LocalAddr:      host.LocalAddr,
		PunchInMS:      PunchDelayMS,
		Role:           "guest",
		RelayID:        hostRelayID,
	}
	// Guest<->guest mesh: pair the new guest with every guest already in
	// the game. Role "peer" tells both sides this is a mesh notification,
	// not a host punch: the client never arms a real punch for it, it just
	// opens NAT mappings (low-TTL first) toward the peer from its lobby and
	// game sockets. PunchInMS 0 because there is no synchronized punch.
	s.removeGuestFromOtherGamesLocked(sess, m.GameID)
	meshPeers := make([]*Session, 0, len(g.guests))
	meshInfos := make([]PeerInfo, 0, len(g.guests))
	// The new guest's relay id as seen by each existing guest (0 when that
	// particular pair lacks mutual relay support).
	meshNewGuestIDs := make([]uint32, 0, len(g.guests))
	for _, other := range g.guests {
		if other == sess {
			continue
		}
		newGuestID, otherID := pairRelayID(sess, other)
		meshPeers = append(meshPeers, other)
		meshInfos = append(meshInfos, PeerInfo{
			Nick:           other.Nick,
			PublicAddr:     other.PublicAddr,
			GamePublicAddr: other.GamePublicAddr,
			LocalAddr:      other.LocalAddr,
			PunchInMS:      0,
			Role:           "peer",
			RelayID:        otherID,
		})
		meshNewGuestIDs = append(meshNewGuestIDs, newGuestID)
	}
	newGuestInfo := PeerInfo{
		Nick:           sess.Nick,
		PublicAddr:     sess.PublicAddr,
		GamePublicAddr: sess.GamePublicAddr,
		LocalAddr:      sess.LocalAddr,
		PunchInMS:      0,
		Role:           "peer",
	}
	inRoster := false
	for _, gg := range g.guests {
		if gg == sess {
			inRoster = true
			break
		}
	}
	if !inRoster {
		g.guests = append(g.guests, sess)
	}
	s.mu.Unlock()

	if err := host.send(MsgPeerInfo, guestInfo); err != nil {
		return fmt.Errorf("host send: %w", err)
	}
	if err := sess.send(MsgPeerInfo, hostInfo); err != nil {
		return fmt.Errorf("guest send: %w", err)
	}
	logger.Printf("matched host=%s(lobby=%s game=%s) <-> guest=%s(lobby=%s game=%s) game=%s",
		hostInfo.Nick, hostInfo.PublicAddr, hostInfo.GamePublicAddr,
		guestInfo.Nick, guestInfo.PublicAddr, guestInfo.GamePublicAddr, m.GameID)
	// Mesh notifications go out AFTER the host/guest pair so the new guest
	// handles its host peer_info (the one that arms the real punch) first.
	// A send error to an existing guest is logged, not fatal: that session
	// is likely dying and will be reaped from the roster on disconnect.
	for i, other := range meshPeers {
		infoForOther := newGuestInfo
		infoForOther.RelayID = meshNewGuestIDs[i]
		if err := other.send(MsgPeerInfo, infoForOther); err != nil {
			logger.Printf("mesh send to %s failed: %v", other.Nick, err)
		}
		if err := sess.send(MsgPeerInfo, meshInfos[i]); err != nil {
			logger.Printf("mesh send to %s failed: %v", sess.Nick, err)
		}
		logger.Printf("mesh peers %s(lobby=%s game=%s) <-> %s(lobby=%s game=%s) game=%s",
			other.Nick, meshInfos[i].PublicAddr, meshInfos[i].GamePublicAddr,
			sess.Nick, newGuestInfo.PublicAddr, newGuestInfo.GamePublicAddr, m.GameID)
	}
	return nil
}

// removeGuestFromOtherGamesLocked drops sess from every game roster except
// keepGameID (pass "" to drop from all). Callers must hold s.mu.
func (s *Server) removeGuestFromOtherGamesLocked(sess *Session, keepGameID string) {
	for gid, g := range s.games {
		if gid == keepGameID {
			continue
		}
		for i, gg := range g.guests {
			if gg == sess {
				g.guests = append(g.guests[:i], g.guests[i+1:]...)
				break
			}
		}
	}
}

func (s *Server) dropSession(sess *Session) {
	// Log the shape of the session on the way out: a client that connected,
	// never got as far as hosting or joining, and vanished seconds later is
	// the fingerprint of a punch/STUN failure on its side.
	logger.Printf("session %s ended nick=%q from %s hosting=%q joining=%q after %s",
		sess.Token[:8], sess.Nick, sess.RemoteAddr, sess.HostingGame, sess.JoiningGame,
		time.Since(sess.Started).Round(time.Second))

	s.mu.Lock()
	delete(s.sessions, sess.Token)
	if sess.HostingGame != "" {
		delete(s.games, sess.HostingGame)
	}
	s.removeGuestFromOtherGamesLocked(sess, "")
	s.mu.Unlock()
}

func (s *Server) handleUDP(udpL *net.UDPConn) {
	s.handleUDPOn(udpL, false)
}

// handleUDPOn: alt=true is the NAT-check listener: it answers STUN probes
// with the observed address but NEVER restamps session/relay state (a
// symmetric NAT maps this destination differently, and adopting that
// mapping would corrupt the addresses handed to peers) and never relays.
func (s *Server) handleUDPOn(udpL *net.UDPConn, alt bool) {
	buf := make([]byte, 2048)
	for {
		n, src, err := udpL.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Printf("udp read: %v", err)
			continue
		}
		// Accept the legacy 20-byte probe (assume lobby) and the current
		// 21-byte probe with a purpose byte.
		if n < 4+SessionTokenBytes {
			continue
		}
		magic := binary.BigEndian.Uint32(buf[0:4])
		if magic != s.Magic {
			continue
		}
		ip4 := src.IP.To4()
		if ip4 == nil {
			continue
		}
		token := hex.EncodeToString(buf[4 : 4+SessionTokenBytes])
		purpose := STUNPurposeLobby
		if n >= STUNRequestSize {
			purpose = buf[4+SessionTokenBytes]
		}
		// Relay data frames share the socket and header; everything after
		// the purpose byte is relay routing plus opaque payload.
		if purpose == RelayPurposeData {
			if !alt && n >= RelayDataHeaderSize {
				s.handleRelayData(udpL, buf[:n], src, token)
			}
			continue
		}
		if purpose != STUNPurposeLobby && purpose != STUNPurposeGame {
			continue
		}
		public := fmt.Sprintf("%s:%d", ip4.String(), src.Port)
		if alt {
			// Reply-only: the observed addr is the whole point of the probe.
			s.mu.Lock()
			_, known := s.sessions[token]
			s.mu.Unlock()
			if !known {
				continue
			}
			resp := make([]byte, STUNResponseSize)
			binary.BigEndian.PutUint32(resp[0:4], s.Magic)
			copy(resp[4:8], ip4)
			binary.BigEndian.PutUint16(resp[8:10], uint16(src.Port))
			udpL.WriteToUDP(resp, src)
			continue
		}
		changed := false
		s.mu.Lock()
		sess, ok := s.sessions[token]
		if ok {
			switch purpose {
			case STUNPurposeGame:
				changed = sess.GamePublicAddr != public
				sess.GamePublicAddr = public
			default:
				changed = sess.PublicAddr != public
				sess.PublicAddr = public
			}
		}
		// STUN probes double as relay return-address updates: they come from
		// the exact sockets relayed traffic must be delivered to, and they
		// keep flowing (as keepalives) while a client idles in a lobby. This
		// also outlives the TCP session, which joiners close at game start.
		if rp, rok := s.relayByToken[token]; rok {
			rp.lastSeen = time.Now()
			srcCopy := *src
			switch purpose {
			case STUNPurposeGame:
				rp.gameAddr = &srcCopy
			default:
				rp.lobbyAddr = &srcCopy
			}
			ok = true
		}
		s.mu.Unlock()
		if !ok {
			logger.Printf("STUN probe with unknown token from %s", src)
			continue
		}

		resp := make([]byte, STUNResponseSize)
		binary.BigEndian.PutUint32(resp[0:4], s.Magic)
		copy(resp[4:8], ip4)
		binary.BigEndian.PutUint16(resp[8:10], uint16(src.Port))
		udpL.WriteToUDP(resp, src)

		// Only when it moves. Clients re-probe on a timer now to keep the
		// NAT mapping from expiring under an idle lobby, so logging every
		// probe would bury the diagnostics in this file under a steady few
		// lines per player per minute. A change is the interesting event
		// anyway: it is what invalidates the address already handed to a
		// peer, and the first probe of a session always counts as one.
		if changed {
			logger.Printf("session %s STUN purpose=%d public=%s", token[:8], purpose, public)
		}
	}
}

// handleRelayData routes one client relay frame: refresh the sender's
// return address for the frame's channel, then (unless it is a keepalive
// with dest 0) rewrite the header to a RelayDeliver and forward it to the
// destination peer's return address. Only pairs introduced via peer_info
// are forwarded, payloads are size- and rate-capped, and every drop reason
// is counted.
func (s *Server) handleRelayData(udpL *net.UDPConn, pkt []byte, src *net.UDPAddr, token string) {
	channel := pkt[4+SessionTokenBytes+1]
	dest := binary.BigEndian.Uint32(pkt[4+SessionTokenBytes+2 : 4+SessionTokenBytes+6])
	payload := pkt[RelayDataHeaderSize:]
	now := time.Now()

	s.mu.Lock()
	rp, ok := s.relayByToken[token]
	if !ok {
		s.mu.Unlock()
		return
	}
	rp.lastSeen = now
	srcCopy := *src
	if channel == RelayChannelGame {
		rp.gameAddr = &srcCopy
	} else {
		rp.lobbyAddr = &srcCopy
	}
	if dest == 0 || len(payload) == 0 {
		// Keepalive / return-address registration only.
		s.mu.Unlock()
		return
	}
	if len(payload) > RelayMaxPayload {
		s.relayDropped++
		s.mu.Unlock()
		return
	}
	// Per-sender rate cap.
	if sec := now.Unix(); sec != rp.rateSec {
		rp.rateSec = sec
		rp.ratePkts = 0
		rp.rateBytes = 0
	}
	rp.ratePkts++
	rp.rateBytes += len(payload)
	if rp.ratePkts > relayMaxPktsPerSec || rp.rateBytes > relayMaxBytesPerSec {
		s.relayDropped++
		s.mu.Unlock()
		return
	}
	target, ok := s.relayPeers[dest]
	if !ok {
		s.relayDropped++
		s.mu.Unlock()
		return
	}
	pair, ok := s.relayPairs[relayPairKey(rp.relayID, dest)]
	if !ok {
		s.relayDropped++
		s.mu.Unlock()
		return
	}
	pair.lastSeen = now
	var dst *net.UDPAddr
	if channel == RelayChannelGame {
		dst = target.gameAddr
	} else {
		dst = target.lobbyAddr
	}
	if dst == nil {
		s.relayDropped++
		s.mu.Unlock()
		return
	}
	firstForward := !pair.logged
	pair.logged = true
	s.relayForwarded++
	s.relayBytes += int64(len(payload))
	srcID := rp.relayID
	srcNick := rp.nick
	dstNick := target.nick
	s.mu.Unlock()

	out := make([]byte, RelayDeliverHeaderSize+len(payload))
	binary.BigEndian.PutUint32(out[0:4], s.Magic)
	out[4] = RelayPurposeDeliver
	out[5] = channel
	binary.BigEndian.PutUint32(out[6:10], srcID)
	copy(out[RelayDeliverHeaderSize:], payload)
	udpL.WriteToUDP(out, dst)

	if firstForward {
		logger.Printf("relay: first frame %s(id %d) -> %s(id %d) ch=%d len=%d",
			srcNick, srcID, dstNick, dest, channel, len(payload))
	}
}

func (s *Server) reapLoop() {
	t := time.NewTicker(ReapInterval)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-SessionTTL)
		s.mu.Lock()
		for tok, sess := range s.sessions {
			if sess.LastSeen.Before(cutoff) {
				logger.Printf("reaping idle session %s", tok[:8])
				sess.Conn.Close()
				delete(s.sessions, tok)
				if sess.HostingGame != "" {
					delete(s.games, sess.HostingGame)
				}
			}
		}
		// UDP relay routing state. Kept independent of sessions on purpose
		// (in-game relayed traffic outlives the joiners' TCP sessions) and
		// refreshed by every frame/probe, so idle really means dead.
		peerCutoff := time.Now().Add(-relayPeerTTL)
		for id, rp := range s.relayPeers {
			if rp.lastSeen.Before(peerCutoff) {
				delete(s.relayPeers, id)
				delete(s.relayByToken, rp.token)
			}
		}
		pairCutoff := time.Now().Add(-relayPairTTL)
		for k, p := range s.relayPairs {
			if p.lastSeen.Before(pairCutoff) {
				delete(s.relayPairs, k)
			}
		}
		// Observer relay slots that never got both attach connections.
		relayCutoff := time.Now().Add(-RelayAttachTTL)
		for tok, slot := range s.relays {
			if slot.created.Before(relayCutoff) {
				logger.Printf("reaping unpaired relay %s", tok[:8])
				if slot.host != nil {
					slot.host.Close()
				}
				if slot.viewer != nil {
					slot.viewer.Close()
				}
				delete(s.relays, tok)
			}
		}
		s.mu.Unlock()
	}
}

func sanitizeNick(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 32 {
		s = s[:32]
	}
	if s == "" {
		s = "anonymous"
	}
	return s
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 64 {
		s = s[:64]
	}
	if s == "" {
		s = "unnamed"
	}
	return s
}

func clampMaxPlayers(n int) int {
	if n < 2 {
		return 2
	}
	if n > 8 {
		return 8
	}
	return n
}
