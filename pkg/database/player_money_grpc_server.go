package database

import (
	"github.com/bill-rich/cncstats/proto/player_money"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PlayerMoneyGRPCServer implements the gRPC PlayerMoneyService server
type PlayerMoneyGRPCServer struct {
	player_money.UnimplementedPlayerMoneyServiceServer
	service *PlayerMoneyService
}

// NewPlayerMoneyGRPCServer creates a new gRPC server for player money service
func NewPlayerMoneyGRPCServer() *PlayerMoneyGRPCServer {
	return &PlayerMoneyGRPCServer{
		service: NewPlayerMoneyService(),
	}
}

// StreamCreatePlayerMoneyData handles bidirectional streaming for creating player money data
func (s *PlayerMoneyGRPCServer) StreamCreatePlayerMoneyData(stream player_money.PlayerMoneyService_StreamCreatePlayerMoneyDataServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			// End of stream
			return nil
		}

		// Convert proto request to internal request
		internalReq := protoToMoneyDataRequest(req)

		// Create the record
		result, err := s.service.CreatePlayerMoneyData(internalReq)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to create player money data: %v", err)
		}

		// Convert result to proto response
		response := playerMoneyDataToProto(result)

		// Send response back to client
		if err := stream.Send(response); err != nil {
			return status.Errorf(codes.Internal, "failed to send response: %v", err)
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

	// Convert money array
	if len(req.Money) == 8 {
		money := [8]int32{}
		copy(money[:], req.Money)
		internalReq.Money = &money
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
		Id:            uint32(data.ID),
		Seed:          data.Seed,
		Timecode:      int32(data.Timecode),
		Player_1Money:  int32(data.Player1Money),
		Player_2Money:  int32(data.Player2Money),
		Player_3Money:  int32(data.Player3Money),
		Player_4Money:  int32(data.Player4Money),
		Player_5Money:  int32(data.Player5Money),
		Player_6Money:  int32(data.Player6Money),
		Player_7Money:  int32(data.Player7Money),
		Player_8Money:  int32(data.Player8Money),
		CreatedAt:     data.CreatedAt.Unix(),
		UpdatedAt:     data.UpdatedAt.Unix(),
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

