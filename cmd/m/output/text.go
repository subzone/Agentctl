package output

import "io"

// Sink wires writers for text or JSON modes.
type Sink struct {
	Out    io.Writer
	Status io.Writer
}
