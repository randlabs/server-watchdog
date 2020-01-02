package guid

import (
	"fmt"
	"strconv"
)

//------------------------------------------------------------------------------

type Guid struct {
	Value1 uint32
	Value2 uint16
	Value3 uint16
	Value4 [8]byte
}

//------------------------------------------------------------------------------

func FromString(s string) (Guid, bool) {
	var guid Guid
	var n uint64
	var err error

	if len(s) == 38 && s[0] == '{' && s[37] == '}' {
		s = s[1:37]
	}
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return Guid{}, false
	}
	n, err = strconv.ParseUint(s[0:8], 16, 32)
	if err != nil {
		return Guid{}, false
	}
	guid.Value1 = uint32(n)

	n, err = strconv.ParseUint(s[9:13], 16, 16)
	if err != nil {
		return Guid{}, false
	}
	guid.Value2 = uint16(n)

	n, err = strconv.ParseUint(s[14:18], 16, 16)
	if err != nil {
		return Guid{}, false
	}
	guid.Value3 = uint16(n)

	ofs := 19
	for i := 0; i < 2; i++ {
		n, err = strconv.ParseUint(s[ofs:(ofs + 2)], 16, 8)
		if err != nil {
			return Guid{}, false
		}
		guid.Value4[i] = byte(n)
		ofs += 2
	}
	ofs++
	for i := 2; i < 8; i++ {
		n, err = strconv.ParseUint(s[ofs:(ofs + 2)], 16, 8)
		if err != nil {
			return Guid{}, false
		}
		guid.Value4[i] = byte(n)
		ofs += 2
	}

	return guid, true
}

func (guid *Guid) ToString() string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X", guid.Value1, guid.Value2, guid.Value3,
						guid.Value4[0], guid.Value4[1], guid.Value4[2], guid.Value4[3], guid.Value4[4], guid.Value4[5],
						guid.Value4[6], guid.Value4[7])
}
