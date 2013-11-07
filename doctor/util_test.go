package doctor

import (
	"testing"
)

func TestOffsetLength(t *testing.T) {
	ol := OffsetLength{offset: 5, length: 20}
	assertEquals("offset 5, length 20", ol.String()/*, t*/)
}
