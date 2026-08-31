package coordinator

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

// testClient is a minimal wire-level client: TCP signaling plus two UDP
// sockets (lobby + game), mirroring what OnlineCoordinatorAPI does.
type testClient struct {
	t       *testing.T
	nick    string
	conn    net.Conn
	r       *bufio.Reader
	token   string
	tokenB  []byte
	magic   uint32
	relayID uint32
	udpPort int

	lobby *net.UDPConn
	game  *net.UDPConn

	lobbyPublic string
	gamePublic  string
}

func (c *testClient) send(t string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		c.t.Fatalf("%s marshal: %v", c.nick, err)
	}
	env := Envelope{Type: t, Data: raw}
	line, _ := json.Marshal(env)
	line = append(line, '\n')
	if _, err := c.conn.Write(line); err != nil {
		c.t.Fatalf("%s write: %v", c.nick, err)
	}
}

// expect reads messages until one of msgType arrives (skipping others),
// unmarshalling its data into out.
func (c *testClient) expect(msgType string, out any) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		c.conn.SetReadDeadline(deadline)
		line, err := c.r.ReadBytes('\n')
		if err != nil {
			c.t.Fatalf("%s waiting for %s: %v", c.nick, msgType, err)
		}
		var env Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			c.t.Fatalf("%s bad envelope: %v", c.nick, err)
		}
		if env.Type == MsgError {
			var e Error
			json.Unmarshal(env.Data, &e)
			c.t.Fatalf("%s got server error while waiting for %s: %s", c.nick, msgType, e.Message)
		}
		if env.Type != msgType {
			continue
		}
		if out != nil {
			if err := json.Unmarshal(env.Data, out); err != nil {
				c.t.Fatalf("%s unmarshal %s: %v", c.nick, msgType, err)
			}
		}
		return
	}
}

// stun sends one STUN probe from the given socket and waits for the
// response, returning the observed public addr.
func (c *testClient) stun(sock *net.UDPConn, purpose byte, serverUDP *net.UDPAddr) string {
	req := make([]byte, STUNRequestSize)
	binary.BigEndian.PutUint32(req[0:4], c.magic)
	copy(req[4:20], c.tokenB)
	req[20] = purpose
	if _, err := sock.WriteToUDP(req, serverUDP); err != nil {
		c.t.Fatalf("%s stun write: %v", c.nick, err)
	}
	buf := make([]byte, 64)
	sock.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err := sock.ReadFromUDP(buf)
	if err != nil {
		c.t.Fatalf("%s stun read: %v", c.nick, err)
	}
	if n != STUNResponseSize {
		c.t.Fatalf("%s stun response size %d", c.nick, n)
	}
	ip := net.IPv4(buf[4], buf[5], buf[6], buf[7])
	port := binary.BigEndian.Uint16(buf[8:10])
	return fmt.Sprintf("%s:%d", ip.String(), port)
}

func (c *testClient) relayFrame(channel byte, dest uint32, payload []byte) []byte {
	frame := make([]byte, RelayDataHeaderSize+len(payload))
	binary.BigEndian.PutUint32(frame[0:4], c.magic)
	copy(frame[4:20], c.tokenB)
	frame[20] = RelayPurposeData
	frame[21] = channel
	binary.BigEndian.PutUint32(frame[22:26], dest)
	copy(frame[RelayDataHeaderSize:], payload)
	return frame
}

func newTestClient(t *testing.T, nick string, tcpAddr string, relay int) *testClient {
	conn, err := net.Dial("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := &testClient{t: t, nick: nick, conn: conn, r: bufio.NewReader(conn)}
	c.send(MsgHello, Hello{Nick: nick, Version: "test", Relay: relay})
	var ok HelloOK
	c.expect(MsgHelloOK, &ok)
	c.token = ok.SessionToken
	c.tokenB, _ = hex.DecodeString(ok.SessionToken)
	c.magic = ok.STUNMagic
	c.relayID = ok.RelayID
	c.udpPort = ok.UDPPort

	c.lobby, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("lobby udp: %v", err)
	}
	c.game, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("game udp: %v", err)
	}
	return c
}

func (c *testClient) list() ([]GameInfo, error) {
	c.send(MsgList, struct{}{})
	var games Games
	c.expect(MsgGames, &games)
	return games.Games, nil
}

func (c *testClient) close() {
	c.conn.Close()
	c.lobby.Close()
	c.game.Close()
}

func startTestServer(t *testing.T) (*Server, string, *net.UDPAddr) {
	s := NewServer()
	// Find free ports the racy-but-fine way.
	tcpL, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tcpAddr := tcpL.Addr().String()
	tcpL.Close()
	udpL, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	udpAddr := udpL.LocalAddr().String()
	udpL.Close()

	// Explicit empty alt-STUN: the production default binds a fixed port,
	// and parallel test servers must not fight over it.
	go s.RunWithAltSTUN(tcpAddr, udpAddr, "")
	// Wait for the listeners to come up.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("tcp", tcpAddr); err == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	serverUDP, err := net.ResolveUDPAddr("udp4", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	return s, tcpAddr, serverUDP
}

func TestRelayEndToEnd(t *testing.T) {
	s, tcpAddr, serverUDP := startTestServer(t)

	host := newTestClient(t, "hostA", tcpAddr, 1)
	defer host.close()
	guest := newTestClient(t, "guestB", tcpAddr, 1)
	defer guest.close()

	if host.relayID == 0 || guest.relayID == 0 {
		t.Fatalf("relay ids not minted: host=%d guest=%d", host.relayID, guest.relayID)
	}
	if host.relayID == guest.relayID {
		t.Fatalf("duplicate relay ids")
	}

	// STUN discovery on all four sockets (also registers relay return addrs).
	host.lobbyPublic = host.stun(host.lobby, STUNPurposeLobby, serverUDP)
	host.gamePublic = host.stun(host.game, STUNPurposeGame, serverUDP)
	guest.lobbyPublic = guest.stun(guest.lobby, STUNPurposeLobby, serverUDP)
	guest.gamePublic = guest.stun(guest.game, STUNPurposeGame, serverUDP)

	// Host lists a game; guest joins; both sides get peer_info with the
	// other's relay id.
	host.send(MsgHost, Host{Name: "g", Map: "m", MaxPlayers: 4,
		LocalAddr: "10.0.0.1:8086", PublicAddr: host.lobbyPublic, GamePublicAddr: host.gamePublic})
	var hosted Hosted
	host.expect(MsgHosted, &hosted)

	guest.send(MsgJoin, Join{GameID: hosted.GameID,
		LocalAddr: "10.0.0.2:8086", PublicAddr: guest.lobbyPublic, GamePublicAddr: guest.gamePublic})
	var guestSeen, hostSeen PeerInfo
	host.expect(MsgPeerInfo, &guestSeen)
	guest.expect(MsgPeerInfo, &hostSeen)
	if guestSeen.RelayID != guest.relayID {
		t.Fatalf("host got peer relay id %d, want %d", guestSeen.RelayID, guest.relayID)
	}
	if hostSeen.RelayID != host.relayID {
		t.Fatalf("guest got peer relay id %d, want %d", hostSeen.RelayID, host.relayID)
	}

	// A frame to a never-introduced id is dropped.
	bogus := guest.relayFrame(RelayChannelLobby, 99999, []byte("nope"))
	guest.lobby.WriteToUDP(bogus, serverUDP)

	// Guest reports a relayed punch outcome; both sides get relay_grant.
	guest.send(MsgPunchOutcome, PunchOutcome{OK: false, Relayed: true, Role: "guest",
		PeerRelayID: host.relayID})
	var gGrant, hGrant RelayGrant
	guest.expect(MsgRelayGrant, &gGrant)
	host.expect(MsgRelayGrant, &hGrant)
	if gGrant.PeerRelayID != host.relayID || hGrant.PeerRelayID != guest.relayID {
		t.Fatalf("grant ids wrong: guest got %d (want %d), host got %d (want %d)",
			gGrant.PeerRelayID, host.relayID, hGrant.PeerRelayID, guest.relayID)
	}

	// Relay a lobby frame guest -> host and a game frame host -> guest.
	checkForward := func(from, to *testClient, fromSock, toSock *net.UDPConn, channel byte, payload string) {
		frame := from.relayFrame(channel, to.relayID, []byte(payload))
		if _, err := fromSock.WriteToUDP(frame, serverUDP); err != nil {
			t.Fatalf("relay write: %v", err)
		}
		buf := make([]byte, 2048)
		toSock.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			n, _, err := toSock.ReadFromUDP(buf)
			if err != nil {
				t.Fatalf("relay deliver read (ch %d): %v", channel, err)
			}
			if n == STUNResponseSize {
				continue // stray STUN reply
			}
			if n < RelayDeliverHeaderSize {
				t.Fatalf("short deliver: %d", n)
			}
			if binary.BigEndian.Uint32(buf[0:4]) != from.magic || buf[4] != RelayPurposeDeliver {
				t.Fatalf("bad deliver header")
			}
			if buf[5] != channel {
				t.Fatalf("channel %d, want %d", buf[5], channel)
			}
			if got := binary.BigEndian.Uint32(buf[6:10]); got != from.relayID {
				t.Fatalf("src id %d, want %d", got, from.relayID)
			}
			if got := string(buf[RelayDeliverHeaderSize:n]); got != payload {
				t.Fatalf("payload %q, want %q", got, payload)
			}
			return
		}
	}
	checkForward(guest, host, guest.lobby, host.lobby, RelayChannelLobby, "hello-lobby")
	checkForward(host, guest, host.game, guest.game, RelayChannelGame, "hello-game")

	// Keepalive (dest 0) refreshes state without forwarding anything.
	ka := guest.relayFrame(RelayChannelGame, 0, nil)
	guest.game.WriteToUDP(ka, serverUDP)

	// The guest's TCP session dying must NOT break relaying (in-game state).
	guest.conn.Close()
	time.Sleep(100 * time.Millisecond)
	checkForward(host, guest, host.game, guest.game, RelayChannelGame, "post-session")

	s.mu.Lock()
	if s.relayForwarded != 3 {
		t.Errorf("relayForwarded = %d, want 3", s.relayForwarded)
	}
	if s.relayDropped == 0 {
		t.Errorf("relayDropped = 0, want >0 (bogus dest)")
	}
	if s.punchRelayed != 1 {
		t.Errorf("punchRelayed = %d, want 1", s.punchRelayed)
	}
	s.mu.Unlock()
}

func TestRelayNotOfferedToOldClients(t *testing.T) {
	_, tcpAddr, serverUDP := startTestServer(t)

	oldc := newTestClient(t, "oldie", tcpAddr, 0)
	defer oldc.close()
	newc := newTestClient(t, "newbie", tcpAddr, 1)
	defer newc.close()

	if oldc.relayID != 0 {
		t.Fatalf("old client got relay id %d", oldc.relayID)
	}

	oldc.lobbyPublic = oldc.stun(oldc.lobby, STUNPurposeLobby, serverUDP)
	oldc.gamePublic = oldc.stun(oldc.game, STUNPurposeGame, serverUDP)
	newc.lobbyPublic = newc.stun(newc.lobby, STUNPurposeLobby, serverUDP)
	newc.gamePublic = newc.stun(newc.game, STUNPurposeGame, serverUDP)

	// Old client hosts; new client joins. Neither peer_info may carry a
	// relay id, because the pair lacks mutual support.
	oldc.send(MsgHost, Host{Name: "g", Map: "m", MaxPlayers: 2,
		LocalAddr: "10.0.0.1:8086", PublicAddr: oldc.lobbyPublic, GamePublicAddr: oldc.gamePublic})
	var hosted Hosted
	oldc.expect(MsgHosted, &hosted)

	newc.send(MsgJoin, Join{GameID: hosted.GameID,
		LocalAddr: "10.0.0.2:8086", PublicAddr: newc.lobbyPublic, GamePublicAddr: newc.gamePublic})
	var oldSees, newSees PeerInfo
	oldc.expect(MsgPeerInfo, &oldSees)
	newc.expect(MsgPeerInfo, &newSees)
	if oldSees.RelayID != 0 || newSees.RelayID != 0 {
		t.Fatalf("mixed-version pair carried relay ids: %d/%d", oldSees.RelayID, newSees.RelayID)
	}
}

func TestAltSTUNAndRestrictedHost(t *testing.T) {
	s := NewServer()
	tcpL, _ := net.Listen("tcp", "127.0.0.1:0")
	tcpAddr := tcpL.Addr().String()
	tcpL.Close()
	udpL, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	udpAddr := udpL.LocalAddr().String()
	udpL.Close()
	udp2L, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	udp2Addr := udp2L.LocalAddr().String()
	udp2L.Close()
	go s.RunWithAltSTUN(tcpAddr, udpAddr, udp2Addr)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("tcp", tcpAddr); err == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	serverUDP, _ := net.ResolveUDPAddr("udp4", udpAddr)

	host := newTestClient(t, "h", tcpAddr, 1)
	defer host.close()
	guest := newTestClient(t, "g", tcpAddr, 1)
	defer guest.close()

	// hello_ok advertises the second port; a probe to it answers with the
	// observed addr and does NOT restamp the session's primary address.
	if host.udpPort == 0 {
		t.Fatal("no udp port")
	}
	s.mu.Lock()
	port2 := s.UDPPort2
	s.mu.Unlock()
	if port2 == 0 {
		t.Fatal("alt STUN port not open")
	}
	altUDP, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("127.0.0.1:%d", port2))

	host.lobbyPublic = host.stun(host.lobby, STUNPurposeLobby, serverUDP)
	host.gamePublic = host.stun(host.game, STUNPurposeGame, serverUDP)
	altSeen := host.stun(host.lobby, STUNPurposeLobby, altUDP)
	if altSeen != host.lobbyPublic {
		t.Fatalf("loopback should observe identical addrs: %s vs %s", altSeen, host.lobbyPublic)
	}
	s.mu.Lock()
	stamped := s.sessions[host.token].PublicAddr
	s.mu.Unlock()
	if stamped != host.lobbyPublic {
		t.Fatalf("alt probe restamped the session addr: %s", stamped)
	}

	guest.lobbyPublic = guest.stun(guest.lobby, STUNPurposeLobby, serverUDP)
	guest.gamePublic = guest.stun(guest.game, STUNPurposeGame, serverUDP)

	host.send(MsgHost, Host{Name: "g", Map: "m", MaxPlayers: 4,
		LocalAddr: "10.0.0.1:8086", PublicAddr: host.lobbyPublic, GamePublicAddr: host.gamePublic})
	var hosted Hosted
	host.expect(MsgHosted, &hosted)
	guest.send(MsgJoin, Join{GameID: hosted.GameID,
		LocalAddr: "10.0.0.2:8086", PublicAddr: guest.lobbyPublic, GamePublicAddr: guest.gamePublic})
	var pi PeerInfo
	guest.expect(MsgPeerInfo, &pi)

	// A relayed guest outcome flags the listing.
	guest.send(MsgPunchOutcome, PunchOutcome{OK: false, Relayed: true, Role: "guest",
		PeerRelayID: pi.RelayID})
	var grant RelayGrant
	guest.expect(MsgRelayGrant, &grant)
	games, err := guest.list()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].RestrictedHost != 1 {
		t.Fatalf("restricted_host not set: %+v", games)
	}
}
