package database

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bill-rich/cncstats/proto/player_money"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// RequestQueueSize is the size of the queue for incoming streaming requests
	RequestQueueSize = 10000
	// QueueSendTimeout is how long to wait when the queue is full before giving up
	QueueSendTimeout = 30 * time.Second
	// WorkerResponseTimeout is how long to wait for a worker to respond
	WorkerResponseTimeout = 30 * time.Second
)

// QueuedRequest represents a request queued for processing
type QueuedRequest struct {
	Request  *player_money.MoneyDataRequest
	Response chan *player_money.MoneyDataResponse
	Error    chan error
}

// PlayerMoneyGRPCServer implements the gRPC PlayerMoneyService server
type PlayerMoneyGRPCServer struct {
	player_money.UnimplementedPlayerMoneyServiceServer
	service      *PlayerMoneyService
	requestQueue chan *QueuedRequest
	workers      sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	once         sync.Once
}

// NewPlayerMoneyGRPCServer creates a new gRPC server for player money service
func NewPlayerMoneyGRPCServer() *PlayerMoneyGRPCServer {
	ctx, cancel := context.WithCancel(context.Background())
	server := &PlayerMoneyGRPCServer{
		service:      NewPlayerMoneyService(),
		requestQueue: make(chan *QueuedRequest, RequestQueueSize),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start worker goroutines to process queued requests
	// Use 10 workers for concurrent processing
	numWorkers := 10
	for i := 0; i < numWorkers; i++ {
		server.workers.Add(1)
		go server.worker()
	}

	return server
}

// worker processes requests from the queue
func (s *PlayerMoneyGRPCServer) worker() {
	defer s.workers.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case queuedReq := <-s.requestQueue:
			if queuedReq == nil {
				return
			}
			s.processRequest(queuedReq)
		}
	}
}

// processRequest handles a single queued request with panic recovery.
func (s *PlayerMoneyGRPCServer) processRequest(queuedReq *QueuedRequest) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("panic", fmt.Sprintf("%v", r)).Error("worker panic recovered")
			queuedReq.Error <- status.Errorf(codes.Internal, "worker panic: %v", r)
			close(queuedReq.Response)
			close(queuedReq.Error)
		}
	}()

	// Convert proto request to internal request
	internalReq := protoToMoneyDataRequest(queuedReq.Request)

	// Create the record
	result, err := s.service.CreatePlayerMoneyData(internalReq)
	if err != nil {
		queuedReq.Error <- status.Errorf(codes.Internal, "failed to create player money data: %v", err)
		close(queuedReq.Response)
		close(queuedReq.Error)
		return
	}

	// Convert result to proto response
	response := playerMoneyDataToProto(result)

	// Send response back
	queuedReq.Response <- response
	close(queuedReq.Response)
	close(queuedReq.Error)
}

// Shutdown gracefully shuts down the server and waits for workers to finish
func (s *PlayerMoneyGRPCServer) Shutdown() {
	s.once.Do(func() {
		s.cancel()
		close(s.requestQueue)
		s.workers.Wait()
	})
}

// StreamCreatePlayerMoneyData handles bidirectional streaming for creating player money data
func (s *PlayerMoneyGRPCServer) StreamCreatePlayerMoneyData(stream player_money.PlayerMoneyService_StreamCreatePlayerMoneyDataServer) error {
	log.Info("stream opened: StreamCreatePlayerMoneyData")
	defer log.Info("stream closed: StreamCreatePlayerMoneyData")

	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.WithError(err).Warn("stream recv error")
			return status.Errorf(codes.Unknown, "recv error: %v", err)
		}

		// Delete existing data for this seed if the client requests it.
		// The client sends this on the first message of a new game session
		// but not on reconnects, avoiding accidental data loss.
		if req.ResetSeed {
			if err := s.service.DeletePlayerMoneyDataBySeed(req.Seed); err != nil {
				log.WithError(err).WithField("seed", req.Seed).Warn("failed to reset seed (continuing)")
			} else {
				log.WithField("seed", req.Seed).Info("reset seed data")
			}
		}

		// Create channels for response and error
		responseChan := make(chan *player_money.MoneyDataResponse, 1)
		errorChan := make(chan error, 1)

		// Create queued request
		queuedReq := &QueuedRequest{
			Request:  req,
			Response: responseChan,
			Error:    errorChan,
		}

		// Try to enqueue the request with backpressure timeout
		enqueued := true
		select {
		case s.requestQueue <- queuedReq:
			// Successfully queued
		case <-s.ctx.Done():
			close(responseChan)
			close(errorChan)
			return status.Errorf(codes.Unavailable, "server is shutting down")
		case <-time.After(QueueSendTimeout):
			// Queue is full after timeout — truly stuck
			close(responseChan)
			close(errorChan)
			log.WithField("seed", req.Seed).Warn("queue send timeout, dropping message")
			enqueued = false
		}

		if !enqueued {
			continue
		}

		// Wait for response or error with timeout
		select {
		case response := <-responseChan:
			if response != nil {
				if err := stream.Send(response); err != nil {
					log.WithError(err).Warn("stream send error (continuing)")
				}
			}
		case err := <-errorChan:
			if err != nil {
				log.WithError(err).Warn("worker error (continuing)")
			}
		case <-s.ctx.Done():
			return status.Errorf(codes.Unavailable, "server is shutting down")
		case <-time.After(WorkerResponseTimeout):
			log.WithField("seed", req.Seed).Warn("worker response timeout, skipping message")
		}
	}
}

// StreamGetPlayerMoneyDataBySeed streams player money data by seed
func (s *PlayerMoneyGRPCServer) StreamGetPlayerMoneyDataBySeed(req *player_money.GetBySeedRequest, stream player_money.PlayerMoneyService_StreamGetPlayerMoneyDataBySeedServer) error {
	if req.Seed == "" {
		return status.Errorf(codes.InvalidArgument, "seed is required")
	}

	results, err := s.service.GetAllPlayerMoneyDataBySeed(req.Seed)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get player money data: %v", err)
	}

	for _, result := range results {
		response := playerMoneyDataToProto(result)
		if err := stream.Send(response); err != nil {
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	return nil
}

// StreamGetAllPlayerMoneyData streams all player money data with pagination
func (s *PlayerMoneyGRPCServer) StreamGetAllPlayerMoneyData(req *player_money.GetAllRequest, stream player_money.PlayerMoneyService_StreamGetAllPlayerMoneyDataServer) error {
	limit := int(req.Limit)
	offset := int(req.Offset)

	results, err := s.service.GetAllPlayerMoneyData(limit, offset)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get player money data: %v", err)
	}

	for _, result := range results {
		response := playerMoneyDataToProto(result)
		if err := stream.Send(response); err != nil {
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	return nil
}

// StreamGetPlayerMoneyDataByTimecode streams player money data by timecode
func (s *PlayerMoneyGRPCServer) StreamGetPlayerMoneyDataByTimecode(req *player_money.GetByTimecodeRequest, stream player_money.PlayerMoneyService_StreamGetPlayerMoneyDataByTimecodeServer) error {
	results, err := s.service.GetPlayerMoneyDataByTimecode(int(req.Timecode))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to get player money data: %v", err)
	}

	for _, result := range results {
		response := playerMoneyDataToProto(result)
		if err := stream.Send(response); err != nil {
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
		}
	}

	return nil
}

// Helper functions to convert between proto and internal types

func protoToMoneyDataRequest(req *player_money.MoneyDataRequest) *MoneyDataRequest {
	internalReq := &MoneyDataRequest{
		Seed:     req.Seed,
		Timecode: req.Timecode,
	}

	// Convert money array - only set if not all zeros
	if len(req.Money) == 8 {
		money := [8]int32{}
		copy(money[:], req.Money)
		// Only set if not all zeros
		if !isAllZerosInt32Array8(money) {
			internalReq.Money = &money
		}
	}

	// Convert other arrays
	if len(req.MoneyEarned) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.MoneyEarned)
		internalReq.MoneyEarned = &arr
	}
	if len(req.UnitsBuilt) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.UnitsBuilt)
		internalReq.UnitsBuilt = &arr
	}
	if len(req.UnitsLost) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.UnitsLost)
		internalReq.UnitsLost = &arr
	}
	if len(req.BuildingsBuilt) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.BuildingsBuilt)
		internalReq.BuildingsBuilt = &arr
	}
	if len(req.BuildingsLost) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.BuildingsLost)
		internalReq.BuildingsLost = &arr
	}
	if len(req.GeneralsPointsTotal) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.GeneralsPointsTotal)
		internalReq.GeneralsPointsTotal = &arr
	}
	if len(req.GeneralsPointsUsed) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.GeneralsPointsUsed)
		internalReq.GeneralsPointsUsed = &arr
	}
	if len(req.RadarsBuilt) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.RadarsBuilt)
		internalReq.RadarsBuilt = &arr
	}
	if len(req.SearchAndDestroy) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.SearchAndDestroy)
		internalReq.SearchAndDestroy = &arr
	}
	if len(req.HoldTheLine) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.HoldTheLine)
		internalReq.HoldTheLine = &arr
	}
	if len(req.Bombardment) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.Bombardment)
		internalReq.Bombardment = &arr
	}
	if len(req.Xp) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.Xp)
		internalReq.XP = &arr
	}
	if len(req.XpLevel) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.XpLevel)
		internalReq.XPLevel = &arr
	}
	if len(req.TechBuildingsCaptured) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.TechBuildingsCaptured)
		internalReq.TechBuildingsCaptured = &arr
	}
	if len(req.FactionBuildingsCaptured) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.FactionBuildingsCaptured)
		internalReq.FactionBuildingsCaptured = &arr
	}
	if len(req.PowerTotal) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.PowerTotal)
		internalReq.PowerTotal = &arr
	}
	if len(req.PowerUsed) == 8 {
		arr := [8]int32{}
		copy(arr[:], req.PowerUsed)
		internalReq.PowerUsed = &arr
	}

	// Convert 2D arrays
	if len(req.BuildingsKilled) == 8 {
		arr := [8][8]int32{}
		for i, row := range req.BuildingsKilled {
			if i < 8 && len(row.GetValues()) == 8 {
				copy(arr[i][:], row.GetValues())
			}
		}
		internalReq.BuildingsKilled = &arr
	}
	if len(req.UnitsKilled) == 8 {
		arr := [8][8]int32{}
		for i, row := range req.UnitsKilled {
			if i < 8 && len(row.GetValues()) == 8 {
				copy(arr[i][:], row.GetValues())
			}
		}
		internalReq.UnitsKilled = &arr
	}

	return internalReq
}

func playerMoneyDataToProto(data *PlayerMoneyData) *player_money.MoneyDataResponse {
	if data == nil {
		return nil
	}

	response := &player_money.MoneyDataResponse{
		Id:        uint32(data.ID),
		Seed:      data.Seed,
		Timecode:  int32(data.Timecode),
		CreatedAt: data.CreatedAt.Unix(),
		UpdatedAt: data.UpdatedAt.Unix(),
	}

	// Convert player_money array
	if data.PlayerMoney.Valid {
		response.PlayerMoney = data.PlayerMoney.Int32Array8[:]
	}

	// Convert nullable arrays
	if data.MoneyEarned.Valid {
		response.MoneyEarned = data.MoneyEarned.Int32Array8[:]
	}
	if data.UnitsBuilt.Valid {
		response.UnitsBuilt = data.UnitsBuilt.Int32Array8[:]
	}
	if data.UnitsLost.Valid {
		response.UnitsLost = data.UnitsLost.Int32Array8[:]
	}
	if data.BuildingsBuilt.Valid {
		response.BuildingsBuilt = data.BuildingsBuilt.Int32Array8[:]
	}
	if data.BuildingsLost.Valid {
		response.BuildingsLost = data.BuildingsLost.Int32Array8[:]
	}
	if data.GeneralsPointsTotal.Valid {
		response.GeneralsPointsTotal = data.GeneralsPointsTotal.Int32Array8[:]
	}
	if data.GeneralsPointsUsed.Valid {
		response.GeneralsPointsUsed = data.GeneralsPointsUsed.Int32Array8[:]
	}
	if data.RadarsBuilt.Valid {
		response.RadarsBuilt = data.RadarsBuilt.Int32Array8[:]
	}
	if data.SearchAndDestroy.Valid {
		response.SearchAndDestroy = data.SearchAndDestroy.Int32Array8[:]
	}
	if data.HoldTheLine.Valid {
		response.HoldTheLine = data.HoldTheLine.Int32Array8[:]
	}
	if data.Bombardment.Valid {
		response.Bombardment = data.Bombardment.Int32Array8[:]
	}
	if data.XP.Valid {
		response.Xp = data.XP.Int32Array8[:]
	}
	if data.XPLevel.Valid {
		response.XpLevel = data.XPLevel.Int32Array8[:]
	}
	if data.TechBuildingsCaptured.Valid {
		response.TechBuildingsCaptured = data.TechBuildingsCaptured.Int32Array8[:]
	}
	if data.FactionBuildingsCaptured.Valid {
		response.FactionBuildingsCaptured = data.FactionBuildingsCaptured.Int32Array8[:]
	}
	if data.PowerTotal.Valid {
		response.PowerTotal = data.PowerTotal.Int32Array8[:]
	}
	if data.PowerUsed.Valid {
		response.PowerUsed = data.PowerUsed.Int32Array8[:]
	}

	// Convert 2D arrays
	if data.BuildingsKilled.Valid {
		for i := 0; i < 8; i++ {
			response.BuildingsKilled = append(response.BuildingsKilled, &player_money.Int32Array8X8{
				Values: data.BuildingsKilled.Int32Array8x8[i][:],
			})
		}
	}
	if data.UnitsKilled.Valid {
		for i := 0; i < 8; i++ {
			response.UnitsKilled = append(response.UnitsKilled, &player_money.Int32Array8X8{
				Values: data.UnitsKilled.Int32Array8x8[i][:],
			})
		}
	}

	return response
}
