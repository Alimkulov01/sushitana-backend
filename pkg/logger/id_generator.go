package logger

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"math/rand/v2"
)

type IDGenerator interface {
	NewLogID(ctx context.Context) LogID
}

type randomIDGenerator struct {
	randSource *rand.ChaCha8
}

var _ IDGenerator = &randomIDGenerator{}

// NewLogID returns a non-zero log ID from a randomly-chosen sequence.
func (gen *randomIDGenerator) NewLogID(context.Context) LogID {
	sid := LogID{}
	for {
		_, _ = gen.randSource.Read(sid[:])
		if sid.IsValid() {
			break
		}
	}
	return sid
}

func defaultIDGenerator() IDGenerator {
	gen := &randomIDGenerator{}
	var seed [32]byte
	_ = binary.Read(crand.Reader, binary.LittleEndian, &seed)
	gen.randSource = rand.NewChaCha8(seed)
	return gen
}
