// Hand-written overrides for the generated protocol bindings. Generated code
// in tsjson.go covers most union/tuple shapes, but the LSP wire format uses
// raw [start, end] JSON arrays for ParameterInformation.label tuples, which
// the generator emits as a struct with Fld0/Fld1 keys. The default decoder
// can't bridge the two shapes, so this file fills the gap.
package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MarshalJSON renders the tuple as a two-element JSON array, matching the
// LSP wire shape.
func (t Tuple_ParameterInformation_label_Item1) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]uint32{t.Fld0, t.Fld1})
}

// UnmarshalJSON accepts the LSP wire shape `[start, end]` for the tuple.
func (t *Tuple_ParameterInformation_label_Item1) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var arr [2]uint32
	if err := dec.Decode(&arr); err != nil {
		return fmt.Errorf("Tuple_ParameterInformation_label_Item1: expected [start, end] JSON array: %w", err)
	}
	t.Fld0 = arr[0]
	t.Fld1 = arr[1]
	return nil
}
