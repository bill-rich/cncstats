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

// TCPAddr and UDPAddr are the listen addresses, overridable via environment
// variables (matching the STATS_DIR/MAPS_DIR convention elsewhere in this
// repo). The game client defaults to TCP 27500 / UDP 27501.
var (
	TCPAddr = ":27500"
	UDPAddr = ":27501"
)

func init() {
	if v := os.Getenv("COORD_TCP_ADDR"); v != "" {
		TCPAddr = v
	}
	if v := os.Getenv("COORD_UDP_ADDR"); v != "" {
		UDPAddr = v
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
	LastSeen       time.Time
	writeMu        sync.Mutex
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
	created  time.Time
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

	mu       sync.Mutex
	sessions map[string]*Session
	games    map[string]*gameState
	relays   map[string]*relayState
	// Lifetime punch telemetry counters (see MsgPunchOutcome).
	punchOK   int
	punchFail int
}

func NewServer() *Server {
	return &Server{
		Magic:    DefaultSTUNMagic,
		sessions: make(map[string]*Session),
		games:    make(map[string]*gameState),
		relays:   make(map[string]*relayState),
	}
}

func (s *Server) Run(tcpAddr, udpAddr string) error {
	tcpL, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return fmt.Errorf("listen tcp: %w", err)
	}
	defer tcpL.Close()
	log.Printf("TCP signaling listening on %s", tcpL.Addr())

	udpResAddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("resolve udp: %w", err)
	}
	udpL, err := net.ListenUDP("udp", udpResAddr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	defer udpL.Close()
	log.Printf("UDP STUN listening on %s", udpL.LocalAddr())
	s.UDPPort = udpL.LocalAddr().(*net.UDPAddr).Port

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
			log.Printf("accept: %v", err)
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
		LastSeen:   time.Now(),
	}

	s.mu.Lock()
	s.sessions[token] = sess
	s.mu.Unlock()
	defer s.dropSession(sess)

	if err := sess.send(MsgHelloOK, HelloOK{
		SessionToken: token,
		STUNMagic:    s.Magic,
		UDPPort:      s.UDPPort,
	}); err != nil {
		return
	}
	log.Printf("session %s nick=%q version=%s from %s", token[:8], sess.Nick, sess.Version, sess.RemoteAddr)

	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("session %s read: %v", token[:8], err)
			}
			return
		}
		s.mu.Lock()
		sess.LastSeen = time.Now()
		s.mu.Unlock()
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			sess.send(MsgError, Error{Message: "bad envelope"})
			continue
		}
		if err := s.handleMessage(sess, &env); err != nil {
			if err == io.EOF {
				return
			}
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
		log.Printf("relay_attach: unknown token from %s", conn.RemoteAddr())
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

	log.Printf("relay_attach: token=%s role=%s from %s (paired=%v)",
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
	log.Printf("relay closed (host=%s viewer=%s)", slot.host.RemoteAddr(), slot.viewer.RemoteAddr())
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
		log.Printf("game started: session=%s game=%s", sess.Token[:8], sess.HostingGame)
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
		s.mu.Lock()
		if m.OK {
			s.punchOK++
		} else {
			s.punchFail++
			// Roll back the optimistic player count from handleJoin so a
			// failed punch (and its retry) doesn't inflate the listing.
			if m.Role == "guest" && sess.JoiningGame != "" {
				if g, ok := s.games[sess.JoiningGame]; ok && g.info.Players > 1 {
					g.info.Players--
				}
			}
		}
		if m.Role == "guest" {
			sess.JoiningGame = ""
		}
		s.mu.Unlock()
		log.Printf("punch outcome session=%s nick=%q role=%s ok=%v lobby=%v game=%v ms=%d",
			sess.Token[:8], sess.Nick, m.Role, m.OK, m.LobbyOK, m.GameOK, m.MS)
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
	log.Printf("session %s hosting game %s name=%q", sess.Token[:8], gameID, m.Name)
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
	log.Printf("observe: viewer=%s game=%s token=%s", sess.Nick, m.GameID, token[:8])
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
	guestInfo := PeerInfo{
		Nick:           sess.Nick,
		PublicAddr:     sess.PublicAddr,
		GamePublicAddr: sess.GamePublicAddr,
		LocalAddr:      sess.LocalAddr,
		PunchInMS:      PunchDelayMS,
		Role:           "host",
	}
	hostInfo := PeerInfo{
		Nick:           host.Nick,
		PublicAddr:     host.PublicAddr,
		GamePublicAddr: host.GamePublicAddr,
		LocalAddr:      host.LocalAddr,
		PunchInMS:      PunchDelayMS,
		Role:           "guest",
	}
	s.mu.Unlock()

	if err := host.send(MsgPeerInfo, guestInfo); err != nil {
		return fmt.Errorf("host send: %w", err)
	}
	if err := sess.send(MsgPeerInfo, hostInfo); err != nil {
		return fmt.Errorf("guest send: %w", err)
	}
	log.Printf("matched host=%s(lobby=%s game=%s) <-> guest=%s(lobby=%s game=%s) game=%s",
		hostInfo.Nick, hostInfo.PublicAddr, hostInfo.GamePublicAddr,
		guestInfo.Nick, guestInfo.PublicAddr, guestInfo.GamePublicAddr, m.GameID)
	return nil
}

func (s *Server) dropSession(sess *Session) {
	s.mu.Lock()
	delete(s.sessions, sess.Token)
	if sess.HostingGame != "" {
		delete(s.games, sess.HostingGame)
	}
	s.mu.Unlock()
}

func (s *Server) handleUDP(udpL *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, src, err := udpL.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			log.Printf("udp read: %v", err)
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
		public := fmt.Sprintf("%s:%d", ip4.String(), src.Port)
		s.mu.Lock()
		sess, ok := s.sessions[token]
		if ok {
			switch purpose {
			case STUNPurposeGame:
				sess.GamePublicAddr = public
			default:
				sess.PublicAddr = public
			}
		}
		s.mu.Unlock()
		if !ok {
			log.Printf("STUN probe with unknown token from %s", src)
			continue
		}

		resp := make([]byte, STUNResponseSize)
		binary.BigEndian.PutUint32(resp[0:4], s.Magic)
		copy(resp[4:8], ip4)
		binary.BigEndian.PutUint16(resp[8:10], uint16(src.Port))
		udpL.WriteToUDP(resp, src)

		log.Printf("session %s STUN purpose=%d public=%s", token[:8], purpose, public)
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
				log.Printf("reaping idle session %s", tok[:8])
				sess.Conn.Close()
				delete(s.sessions, tok)
				if sess.HostingGame != "" {
					delete(s.games, sess.HostingGame)
				}
			}
		}
		// Observer relay slots that never got both attach connections.
		relayCutoff := time.Now().Add(-RelayAttachTTL)
		for tok, slot := range s.relays {
			if slot.created.Before(relayCutoff) {
				log.Printf("reaping unpaired relay %s", tok[:8])
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

// Status is a read-only snapshot for the HTTP status endpoint.
type Status struct {
	Sessions  int        `json:"sessions"`
	Games     []GameInfo `json:"games"`
	PunchOK   int        `json:"punch_ok"`
	PunchFail int        `json:"punch_fail"`
}

func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{
		Sessions:  len(s.sessions),
		Games:     make([]GameInfo, 0, len(s.games)),
		PunchOK:   s.punchOK,
		PunchFail: s.punchFail,
	}
	for _, g := range s.games {
		st.Games = append(st.Games, g.info)
	}
	return st
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
