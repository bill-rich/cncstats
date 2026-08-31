package coordinator

// Status is a read-only snapshot for an HTTP status endpoint (cncstats
// serves it at /coordinator/status; the standalone coord serves it on the
// -status listen address). The relay counters are the primary telemetry for
// the relay-fallback test harness: relay_forwarded/relay_bytes prove which
// runs actually relayed, relay_dropped catches misrouted or rate-limited
// traffic, and punch_relayed counts pairs that fell back.
type Status struct {
	Sessions  int        `json:"sessions"`
	Games     []GameInfo `json:"games"`
	PunchOK   int        `json:"punch_ok"`
	PunchFail int        `json:"punch_fail"`
	// UDP relay fallback telemetry.
	PunchRelayed    int   `json:"punch_relayed"`
	RelayPeers      int   `json:"relay_peers"`
	RelayPairs      int   `json:"relay_pairs"`
	RelayForwarded  int64 `json:"relay_forwarded"`
	RelayBytes      int64 `json:"relay_bytes"`
	RelayDropped    int64 `json:"relay_dropped"`
	RelayGrantsSent int   `json:"relay_grants_sent"`
}

func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{
		Sessions:        len(s.sessions),
		Games:           make([]GameInfo, 0, len(s.games)),
		PunchOK:         s.punchOK,
		PunchFail:       s.punchFail,
		PunchRelayed:    s.punchRelayed,
		RelayPeers:      len(s.relayPeers),
		RelayPairs:      len(s.relayPairs),
		RelayForwarded:  s.relayForwarded,
		RelayBytes:      s.relayBytes,
		RelayDropped:    s.relayDropped,
		RelayGrantsSent: s.relayGrantsSent,
	}
	for _, g := range s.games {
		st.Games = append(st.Games, g.info)
	}
	return st
}
