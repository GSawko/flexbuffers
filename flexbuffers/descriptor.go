package flexbuffers

import (
	"math"
)

type ByteSize uint8

const (
	b8  ByteSize = iota // 2^0 = 1 byte
	b16                 // 2^1 = 2 bytes
	b32                 // 2^2 = 4 bytes
	b64                 // 2^3 = 8 bytes
)

type VarType uint8

const (
	NULL  VarType = 0
	INT           = 1
	UINT          = 2
	FLOAT         = 3
	//Types above stored inline, types below store an offset.
	KEY            = 4
	STRING         = 5
	INDIRECT_INT   = 6
	INDIRECT_UINT  = 7
	INDIRECT_FLOAT = 8
	MAP            = 9
	VECTOR         = 10 //Untyped.

	VECTOR_INT   = 11 // Typed any size (stores no type table).
	VECTOR_UINT  = 12
	VECTOR_FLOAT = 13
	VECTOR_KEY   = 14
	// DEPRECATED, use VECTOR or VECTOR_KEY instead.
	// Read test.cpp/FlexBuffersDeprecatedTest() for details on why.
	VECTOR_STRING_DEPRECATED = 15

	VECTOR_INT2   = 16 // Typed tuple (no type table, no size field).
	VECTOR_UINT2  = 17
	VECTOR_FLOAT2 = 18
	VECTOR_INT3   = 19 // Typed triple (no type table, no size field).
	VECTOR_UINT3  = 20
	VECTOR_FLOAT3 = 21
	VECTOR_INT4   = 22 // Typed quad (no type table, no size field).
	VECTOR_UINT4  = 23
	VECTOR_FLOAT4 = 24

	BLOB        = 25
	BOOL        = 26
	VECTOR_BOOL = 36 // To do the same type of conversion of type to vector type
)

func (v VarType) toString() string {
	switch v {
	case NULL:
		return "NULL"
	case INT:
		return "INT"
	case UINT:
		return "UINT"
	case FLOAT:
		return "FLOAT"
	case KEY:
		return "KEY"
	case STRING:
		return "STRING"
	case INDIRECT_INT:
		return "INDIRECT_INT"
	case INDIRECT_UINT:
		return "INDIRECT_UINT"
	case INDIRECT_FLOAT:
		return "INDIRECT_FLOAT"
	case MAP:
		return "MAP"
	case VECTOR:
		return "VECTOR"
	case VECTOR_INT:
		return "VECTOR_INT"
	case VECTOR_UINT:
		return "VECTOR UINT"
	case VECTOR_FLOAT:
		return "VECTOR_FLOAT"
	case VECTOR_KEY:
		return "VECTOR_KEY"
	case VECTOR_STRING_DEPRECATED:
		return "VECTOR_STRING_DEPRECATED"
	case VECTOR_INT2:
		return "VECTOR_INT2"
	case VECTOR_UINT2:
		return "VECTOR_UINT2"
	case VECTOR_FLOAT2:
		return "VECTOR_FLOAT2"
	case VECTOR_INT3:
		return "VECTOR_INT3"
	case VECTOR_UINT3:
		return "VECTOR_UINT3"
	case VECTOR_FLOAT3:
		return "VECTOR_FLOAT3"
	case VECTOR_INT4:
		return "VECTOR_INT4"
	case VECTOR_UINT4:
		return "VECTOR_UINT4"
	case VECTOR_FLOAT4:
		return "VECTOR_FLOAT4"
	case BLOB:
		return "BLOB"
	case BOOL:
		return "BOOL"
	case VECTOR_BOOL:
		return "VECTOR_BOOL"
	default:
		return "Unknown type"
	}
}

func uintSize(value uint64) int {
	if value < (1 << 8) {
		return 1
	} else if value < (1 << 16) {
		return 2
	} else if value < (1 << 32) {
		return 4
	} else {
		return 8
	}
}

func intSize(value int64) int {
	if value >= -(1<<7) && value < 1<<7 {
		return 1
	} else if value >= -(1<<15) && value < 1<<15 {
		return 2
	} else if value >= -(1<<31) && value < 1<<31 {
		return 4
	} else {
		return 8
	}
}

func floatSize(value float64) int {
	if float64(float32(value)) == value {
		return 4
	}
	return 8
}

func b(byte_width int) ByteSize {
	if byte_width < 0 || byte_width > 8 {
		panic("Too large field size to encode")
	}
	l := math.Ceil(math.Log2(float64(byte_width)))
	return ByteSize(l)
}

func B(bs ByteSize) uint64 {
	return uint64(math.Pow(2, float64(bs)))
}

// Describes a variable in terms of byte size and type
// 2 last bits describe the size according to the ByteSize enum(00=8bits,01=16 bits,10=32bits,11=64 bits)
// the remaining bits describe the type according to the VarType enum
type context uint8

func Pack(vType VarType, bSize ByteSize) context {
	return context(uint(vType)<<2 | uint(bSize))
}

func (d *context) ItemVarType() VarType {
	return VarType(*d >> 2)
}

func (d *context) ItemByteSize() ByteSize {
	return ByteSize(*d) & 3 //&00000011
}
