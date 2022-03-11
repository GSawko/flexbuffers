package flexbuffers

import (
	"bytes"
)

type Offset uint32

func isInline(vType VarType) bool { //NULL,INT,UINT,FLOAT,BOOL
	return vType <= 3 || vType == 26
}

func isTyped(vType VarType) bool {
	return isTypedVector(vType) || isBlobLike(vType)
}

func isTypedVector(vType VarType) bool {
	return vType > 10 && vType < 16 || isFixedTypedVector(vType) || vType == VECTOR_BOOL || vType == VECTOR_KEY
}

func isVector(vType VarType) bool {
	return vType == VECTOR || isTypedVector(vType) || isFixedTypedVector(vType)
}

func isBlobLike(vType VarType) bool {
	return vType == BLOB || vType == STRING || vType == KEY
}

func isFixedTypedVector(vType VarType) bool { //offset scalars,tuples,triples,quads
	return isScalar(vType) || isTuple(vType) || isTriple(vType) || isQuad(vType)
}

func isScalar(vType VarType) bool {
	return vType >= 6 && vType <= 8
}

func isTuple(vType VarType) bool {
	return vType >= 16 && vType <= 18
}

func isTriple(vType VarType) bool {
	return vType >= 19 && vType <= 21
}

func isQuad(vType VarType) bool {
	return vType >= 22 && vType <= 24
}

func isIntTyped(vType VarType) bool {
	return vType == VECTOR_INT || vType == INDIRECT_INT || vType == VECTOR_INT2 || vType == VECTOR_INT3 || vType == VECTOR_INT4
}

func isUintTyped(vType VarType) bool {
	return vType == VECTOR_UINT || vType == INDIRECT_UINT || vType == VECTOR_UINT2 || vType == VECTOR_UINT3 || vType == VECTOR_UINT4 || isBlobLike(vType)
}

func isFloatTyped(vType VarType) bool {
	return vType == VECTOR_FLOAT || vType == INDIRECT_FLOAT || vType == VECTOR_FLOAT2 || vType == VECTOR_FLOAT3 || vType == VECTOR_FLOAT4
}

func isBoolTyped(vType VarType) bool {
	return vType == VECTOR_BOOL
}

type Decoder struct {
	Reader bytes.Buffer
}

func Marshal(item interface{}) ([]byte, error) {
	b := NewBuilder()
	if err := b.AutoBuild(item); err != nil {
		return nil, err
	}
	buff := []byte{}
	_, err := b.SerializeBuffer(&buff)
	return buff, err
}
