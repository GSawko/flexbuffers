package flexbuffers

import (
	"encoding/binary"
	"fmt"
	"math"
)

type elementHandler interface {
	insertOffsetToObject(iStructure, int) (int, error) //defines policy for adding offsets
	insertElement(element, int) (int, error)           //defines policy for adding all types of elements
	updateBitWidth()
	serializeElems(*[]byte) (int, error)
	elemsCount() int
}

type offsetHandler interface {
	bindOffset(off *element)
	updateOffsets(index0 uint64) error
	serializeChildren(*[]byte) (int, error)
}

type iStructure interface { //eg. flexMap,vector,typedVector, Scalars,Tuples,Triples etc...
	elementHandler
	offsetHandler
	getVtype() VarType
	getBsize() ByteSize
}

//An element must be embedded inside a structure and directly represents data. This includes offsets to structures
//Implements serializable
type element struct {
	bytes      [8]byte
	absIndex   uint64 //stores the absolute index of an element. For offsets stores the absolute index of target
	fieldType  VarType
	fieldSize  ByteSize
	targetSize ByteSize //for offsets - stores the bit width of the target structure. For other elements - remains empty
}

func newUINT(u uint64) element {
	var o element
	//TODO: optimize
	binary.LittleEndian.PutUint64(o.bytes[:], u)
	o.fieldType = UINT
	o.fieldSize = b(uintSize(u))
	return o
}

func newINT(i int64) element {
	var o element
	//TODO: optimize
	binary.LittleEndian.PutUint64(o.bytes[:], uint64(i))
	o.fieldType = INT
	o.fieldSize = b(intSize(i))
	return o
}

func newFLOAT(f float64) element {
	var o element
	s := floatSize(f)
	if s == 4 {
		binary.LittleEndian.PutUint32(o.bytes[:], math.Float32bits(float32(f)))
	} else {
		binary.LittleEndian.PutUint64(o.bytes[:], math.Float64bits(f))
	}
	o.fieldType = FLOAT
	o.fieldSize = b(s)
	return o
}

func newBOOL(b bool) element {
	var o element
	//TODO: optimize
	if b {
		o.bytes[0] = 1
	}
	o.fieldType = BOOL
	o.fieldSize = ByteSize(0)
	return o
}

func newNULL() element {
	var o element
	o.fieldType = NULL
	o.fieldSize = ByteSize(0)
	return o
}

func newOffset(index0 uint64, targetType VarType, targetSize ByteSize) element {
	//TODO: ensure vType is of structure type
	var o element
	//TODO: optimize
	binary.LittleEndian.PutUint64(o.bytes[:], index0)
	o.fieldType = targetType
	o.targetSize = targetSize
	o.fieldSize = ByteSize(uintSize(index0))
	o.absIndex = index0
	return o
}

func (e element) serialize(buff *[]byte, args ...interface{}) (int, error) {
	n := len(*buff)
	if len(args) == 0 {
		return n, fmt.Errorf("unable to serialize an element: Field size not provided")
	}
	byteWidth, ok := args[0].(ByteSize)
	if !ok {
		return n, fmt.Errorf("unable to serialize an element: Expected parameter of type ByteSize but got %T", args[0])
	}
	*buff = append(*buff, e.bytes[:B(byteWidth)]...)
	e.absIndex = uint64(n)
	return n, nil
}

//A structure is an offset object that may contain inline objects, including offsets to other structures
//implements elementHandler, offsetHandler, serializable
type structure struct {
	offsetPtrs []*element //can bind to multiple offsets
	elems      []*element
	children   []iStructure
	vType      VarType  //TODO: remove redundant member
	bSize      ByteSize //maximum element size
	//maybe bWidth should be a special enum type
}

func (s *structure) elemsCount() int {
	return len(s.elems)
}

func (s *structure) bindOffset(off *element) {
	if off == nil {
		panic("unable to bind to a non-existent element")
	}
	s.offsetPtrs = append(s.offsetPtrs, off)
}

func (s *structure) getVtype() VarType {
	return s.vType
}

func (s *structure) getBsize() ByteSize {
	return s.bSize
}

func serialize(s iStructure, buff *[]byte, apply_padding bool) (int, error) {
	if i, err := s.serializeChildren(buff); err != nil {
		return i, err
	}
	i, err := s.serializeElems(buff)
	if err != nil {
		return i, err
	}
	if s.elemsCount() != 0 && apply_padding {
		appendPadding(buff, int(B(s.getBsize())))
	}
	return i, nil
}

func addElement(s iStructure, elem element) (int, error) {
	return s.insertElement(elem, s.elemsCount())
}

func addOffsetToObject(s iStructure, obj iStructure) (int, error) {
	return s.insertOffsetToObject(obj, s.elemsCount())
}

func (s *structure) updateOffsets(index0 uint64) error {
	for _, sp := range s.offsetPtrs {
		sp.absIndex = index0
		sp.fieldType = s.vType
		sp.targetSize = s.bSize
	}
	return nil
}

func (s *structure) updateBitWidth() {
	s.bSize = 0
	for _, elem := range s.elems {
		if elem.fieldSize > s.bSize {
			s.bSize = elem.fieldSize
		}
	}
}

//HELP: flatc does not apply padding before root
func appendPadding(buff *[]byte, scalarSize int) {
	bufSize := len(*buff)
	padding := (^bufSize + 1) & (scalarSize - 1)
	/*word_size := 4
	bytes := len(*buff)
	padding := (word_size - (bytes % word_size)) % word_size*/
	if padding != 0 {
		*buff = append(*buff, make([]byte, padding)...)
	}

}

func (s *structure) serializeChildren(buff *[]byte) (int, error) {
	for _, ch := range s.children {
		i, err := serialize(ch, buff, true)
		if err != nil {
			return i, err
		}
	}
	s.updateBitWidth()
	return len(*buff), nil
}

func (s *structure) serializeElems(buff *[]byte) (int, error) {
	index0 := len(*buff)
	for i, elem := range s.elems {
		if !isInline(elem.fieldType) {
			*s.elems[i] = newOffset(uint64(index0+i-int(elem.absIndex)), elem.fieldType, elem.targetSize)
		}
		i, err := s.elems[i].serialize(buff, s.bSize)
		if err != nil {
			return i, err
		}
	}
	err := s.updateOffsets(uint64(index0)) //TODO: fix problem with key vector not having any offsets to update
	return index0, err
}

func (s *structure) allowsElemType(elemT VarType) bool {
	switch s.vType {
	case VECTOR_UINT, INDIRECT_UINT, VECTOR_UINT2, VECTOR_UINT3, VECTOR_UINT4, BLOB, KEY, STRING:
		return elemT == UINT
	case VECTOR_INT, INDIRECT_INT, VECTOR_INT2, VECTOR_INT3, VECTOR_INT4:
		return elemT == INT
	case VECTOR_FLOAT, INDIRECT_FLOAT, VECTOR_FLOAT2, VECTOR_FLOAT3, VECTOR_FLOAT4:
		return elemT == FLOAT
	case VECTOR_BOOL:
		return elemT == BOOL
	case VECTOR_KEY:
		return elemT == KEY
	case VECTOR_STRING_DEPRECATED:
		return elemT == STRING
	}
	return true
}

func (s *structure) allowsElemSize(elemS ByteSize) bool {
	switch s.vType {
	case BLOB:
		return elemS == b8
	case KEY:
		return elemS == b8
	case STRING:
		return elemS == b8
	}
	return true
}

func (s *structure) allows(elem *element) error {
	elemT := elem.fieldType
	elemS := elem.fieldSize
	if !s.allowsElemType(elemT) {
		return fmt.Errorf("unable to add element of type %s to a structure of type %s", elemT.toString(), s.vType.toString())
	}
	if !s.allowsElemSize(elemS) {
		return fmt.Errorf("unable to add element of size %d byte(s) to a structure of type %s", int(elemS), s.vType.toString())
	}
	if s.isFull() {
		return fmt.Errorf("unable to add any more elements: structure of type %s is full", s.vType.toString())
	}
	return nil
}

func (s *structure) isFull() bool {
	n := len(s.elems)
	switch s.vType {
	case INDIRECT_INT, INDIRECT_UINT, INDIRECT_FLOAT:
		return n >= 1
	case VECTOR_INT2, VECTOR_UINT2, VECTOR_FLOAT2:
		return n >= 2
	case VECTOR_INT3, VECTOR_UINT3, VECTOR_FLOAT3:
		return n >= 3
	case VECTOR_INT4, VECTOR_UINT4, VECTOR_FLOAT4:
		return n >= 4

	}
	return false
}

func (s *structure) addElement(elem element) (int, error) {
	return s.insertElement(elem, len(s.elems))
}

func (s *structure) addOffsetToObject(o iStructure) (int, error) {
	return s.insertOffsetToObject(o, len(s.elems))
}

func (s *structure) insertElement(e element, index int) (int, error) {
	if err := s.allows(&e); err != nil {
		return -1, err
	}
	// insert at the specified index if possible. Otherwise insert at the end
	length := len(s.elems)
	if index <= length-1 {
		s.elems = append(s.elems[:index+1], s.elems[index:]...)
		s.elems[index] = &e
	} else {
		s.elems = append(s.elems, &e)
		index = length
	}
	return index, nil
}

func (s *structure) insertOffsetToObject(o iStructure, index int) (int, error) {
	offset := element{fieldType: o.getVtype()}
	n, err := s.insertElement(offset, index)
	if err != nil {
		return n, err
	}
	o.bindOffset(s.elems[n])
	//insert the structure at the same index as the offset(if possible). Otherwise insert at the end
	length := len(s.children)
	if n <= length-1 {
		s.children = append(s.children[:n+1], s.children[n:]...)
		s.children[n] = o
	} else {
		s.children = append(s.children, o)
		n = length
	}
	return n, nil
}

type typedVector struct {
	structure
}

func newTypedVector(vType VarType) *typedVector {
	if !isTypedVector(vType) {
		panic(fmt.Sprintf("Can not create a typed vector of type %s", vType.toString()))
	}
	v := new(typedVector)
	v.vType = vType
	return v
}

func (tv *typedVector) serializeElems(buff *[]byte) (int, error) {
	//prepend vector size
	n := uint64(len(tv.elems))
	//TODO: optimize
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, n)
	bytes = bytes[:B(tv.bSize)]
	*buff = append(*buff, bytes...)
	return tv.structure.serializeElems(buff)
}

type vector struct {
	typedVector
}

func newVector() *vector {
	v := new(vector)
	v.vType = VECTOR
	return v
}

func (v *vector) serializeElems(buff *[]byte) (int, error) {

	index0, err := v.typedVector.serializeElems(buff)
	if err != nil {
		return index0, err
	}
	//append item descriptors at the end
	for _, elem := range v.elems {
		var desc context
		if isInline(elem.fieldType) {
			desc = Pack(elem.fieldType, v.bSize)
		} else {
			desc = Pack(elem.fieldType, elem.targetSize)
		}
		*buff = append(*buff, byte(desc))
	}
	return index0, err
}

type fixedTypedVector struct {
	typedVector
}

func newFixedTypedVector(vType VarType) *fixedTypedVector {
	if !isFixedTypedVector(vType) {
		panic(fmt.Sprintf("Can not create a fixed typed vector of type %s", vType.toString()))
	}
	v := new(fixedTypedVector)
	v.vType = vType
	return v
}

type blob struct {
	typedVector
}

func newBlob(bytes []byte) *blob {
	v := new(blob)
	v.vType = BLOB
	v.bSize = b8
	return v
}

func (s *blob) Append(bytes []byte) {
	for _, b := range bytes {
		s.addElement(newUINT(uint64(b)))
	}
}

type flexString struct {
	blob
}

func newFlexString(str string) *flexString {
	v := new(flexString)
	v.vType = STRING
	v.bSize = b8
	v.Append(str)
	return v
}

func (s *flexString) serializeElems(buff *[]byte) (int, error) {
	n, err := s.blob.serializeElems(buff)
	*buff = append(*buff, byte(0)) //append 0-termination byte
	return n, err
}

func (s *flexString) Append(str string) {
	for _, char := range str {
		s.addElement(newUINT(uint64(char)))
	}
}

func (s *flexString) Equals(other *flexString) bool {
	if len(s.elems) != len(other.elems) {
		return false
	}
	for i := range s.elems {
		if s.elems[i] != other.elems[i] {
			return false
		}
	}
	return true
}

func (s *flexString) toString() string {
	str := ""
	for _, elem := range s.elems {
		str += string(elem.bytes[0])
	}
	return str
}

type key struct {
	flexString
}

func newKey(str string) *key {
	v := new(key)
	v.vType = KEY
	v.bSize = b8
	v.Append(str)
	return v
}

func (k *key) serializeElems(buff *[]byte) (int, error) {
	n, err := k.structure.serializeElems(buff)
	*buff = append(*buff, byte(0)) //append 0-termination byte
	return n, err
}

type keyVector struct {
	typedVector
}

func newKeyVector() *keyVector {
	v := new(keyVector)
	v.vType = VECTOR_KEY
	return v
}

type flexMap struct {
	vector
	keys *keyVector
}

func newFlexMap(args ...interface{}) *flexMap {
	v := new(flexMap)
	v.vType = MAP
	if len(args) > 0 {
		if val, ok := args[0].(*keyVector); ok {
			v.keys = val
			return v
		}
		panic(fmt.Sprintf("Expected optional argument of type keyVector, got %T", args[0]))
	}
	v.keys = newKeyVector()
	return v
}

func (m *flexMap) insertOffsetToObject(o iStructure, index int) (int, error) {
	return -1, fmt.Errorf("can not insert offset without key into map. Use addOffsetWithKey(*key,iStructure)")
}

func (m *flexMap) addOffsetToObject(o iStructure) (int, error) {
	return m.insertOffsetToObject(o, len(m.elems))
}

func (m *flexMap) insertElement(e element, index int) (int, error) {
	return -1, fmt.Errorf("can not insert element without key into map. Use addElementWithKey(*key,element)")
}

func (m *flexMap) addElement(o iStructure) (int, error) {
	return m.insertOffsetToObject(o, len(m.elems))
}

func (m *flexMap) containsKey(k *key) bool {
	for _, c := range m.keys.children {
		K := c.(*key)
		if K.Equals(&k.flexString) {
			return true
		}
	}
	return false
}

func (m *flexMap) determineKeyInsertionIndex(newkey *key) int {
	var DAC func([]iStructure) int
	DAC = func(subslice []iStructure) int {
		length := len(subslice)
		if length == 0 {
			return 0
		} else if length == 1 {
			key := subslice[0].(*key)
			if newkey.toString() < key.toString() {
				return 0
			} else {
				return 1
			}
		} else {
			pivot := length / 2
			key := subslice[pivot].(*key)
			if newkey.toString() < key.toString() {
				return DAC(subslice[:pivot])
			} else {
				ss := subslice[pivot+1:]
				return length - len(ss) + DAC(ss)
			}
		}
	}
	return DAC(m.keys.children)
}

func (m *flexMap) addElementWithKey(k *key, e element) (int, error) {
	if m.containsKey(k) {
		return -1, fmt.Errorf("the key %s already exists within the map - duplicate keys are not allowed", k.toString())
	}
	index := m.determineKeyInsertionIndex(k)
	n, err := m.keys.insertOffsetToObject(k, index)
	if err != nil {
		return n, err
	}
	return m.structure.insertElement(e, n)
}

func (m *flexMap) addOffsetWithKey(k *key, o iStructure) (int, error) {
	if m.containsKey(k) {
		return -1, fmt.Errorf("the key already exists within the map - duplicate keys are not allowed")
	}
	index := m.determineKeyInsertionIndex(k)
	n, err := m.keys.insertOffsetToObject(k, index)
	if err != nil {
		return n, err
	}
	return m.structure.insertOffsetToObject(o, index)
}

func (m *flexMap) serializeChildren(buff *[]byte) (int, error) {
	if i, err := m.structure.serializeChildren(buff); err != nil {
		return i, err
	}
	i, err := serialize(m.keys, buff, true)
	if err != nil {
		return i, err
	}
	kbw := b(uintSize(uint64(i)))
	if kbw > m.bSize {
		m.bSize = kbw
	}
	//TODO: optimize
	//append offset to keys vector
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(len(*buff)-i))
	bytes = bytes[:B(m.bSize)]
	*buff = append(*buff, bytes...)
	//append key vector byte width
	bytes = make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, uint64(B(m.keys.bSize)))
	bytes = bytes[:B(m.bSize)]
	*buff = append(*buff, bytes...)
	return len(*buff), nil
}

type root struct {
	structure
}

func (r *root) serializeChildren(buff *[]byte) (int, error) {
	i := len(*buff)
	if len(r.children) != 0 {
		ch := r.children[0]
		i, err := serialize(ch, buff, false)
		if err != nil {
			return i, err
		}
	}
	r.updateBitWidth()
	return i, nil
}

func (r *root) serializeElems(buff *[]byte) (int, error) {
	i, err := r.structure.serializeElems(buff)
	if err != nil {
		return i, err
	}
	// append element context
	if isInline(r.elems[0].fieldType) {
		*buff = append(*buff, byte(Pack(r.elems[0].fieldType, r.bSize)))
	} else {
		*buff = append(*buff, byte(Pack(r.elems[0].fieldType, r.elems[0].targetSize)))
	}
	//append bitwidth
	*buff = append(*buff, byte(B(r.bSize)))
	return i, err
}

func (r *root) insertElement(elem element, index int) (int, error) {
	if len(r.elems) > 0 {
		return -1, fmt.Errorf("can not insert more than 1 element to root")
	}
	return r.structure.insertElement(elem, 0)
}

func (r *root) insertOffsetToObject(o iStructure, index int) (int, error) {
	if len(r.children) > 0 {
		return -1, fmt.Errorf("can not insert more than 1 child structure to root")
	}
	return r.structure.insertOffsetToObject(o, 0)
}
