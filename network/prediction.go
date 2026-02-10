package network

import "github.com/automoto/doomerang-mp/shared/messages"

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

