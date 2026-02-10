package network

import (
	"math"

	"github.com/automoto/doomerang-mp/shared/messages"
)

const predictionBufferSize = 64

// InputRecord stores an input alongside the predicted position after applying it.
type InputRecord struct {
	Input      messages.PlayerInput
	PredictedX float64
	PredictedY float64
}

// PredictionBuffer is a ring buffer that stores recent inputs and their
// predicted outcomes for server reconciliation.
type PredictionBuffer struct {
	history [predictionBufferSize]InputRecord
	nextSeq uint32
}

// Store saves an input and the resulting predicted position.
func (pb *PredictionBuffer) Store(input messages.PlayerInput, predX, predY float64) {
	idx := input.Sequence % predictionBufferSize
	pb.history[idx] = InputRecord{
		Input:      input,
		PredictedX: predX,
		PredictedY: predY,
	}
	pb.nextSeq = input.Sequence + 1
}

// Get retrieves a stored record by sequence number. Returns false if not found
// or if the slot has been overwritten.
func (pb *PredictionBuffer) Get(seq uint32) (InputRecord, bool) {
	idx := seq % predictionBufferSize
	record := pb.history[idx]
	if record.Input.Sequence != seq {
		return InputRecord{}, false
	}
	return record, true
}

// NextSeq returns the next expected sequence number.
func (pb *PredictionBuffer) NextSeq() uint32 {
	return pb.nextSeq
}

// GetUnacknowledged returns all stored inputs with sequence numbers greater
// than lastAcked and less than nextSeq (i.e. inputs the server hasn't
// confirmed yet).
func (pb *PredictionBuffer) GetUnacknowledged(lastAcked uint32) []InputRecord {
	var results []InputRecord
	for seq := lastAcked + 1; seq < pb.nextSeq; seq++ {
		if record, ok := pb.Get(seq); ok {
			results = append(results, record)
		}
	}
	return results
}

// PredictionError calculates the distance between predicted and actual server
// position for a given sequence.
func (pb *PredictionBuffer) PredictionError(seq uint32, serverX, serverY float64) float64 {
	record, ok := pb.Get(seq)
	if !ok {
		return 0
	}
	dx := record.PredictedX - serverX
	dy := record.PredictedY - serverY
	return math.Sqrt(dx*dx + dy*dy)
}
