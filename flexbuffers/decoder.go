package flexbuffers

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"
)

type Ref struct { //interprets raw data
	buffer []byte
	context
	index_0    uint64
	item_count uint64
}

//TODO: is int64 too big?
//TODO convert all from  Ref to Ref
//for finding null bytes : package bytes, function IndexByte
func (r Ref) getItemAsRef(item_index int64, con context) Ref {
	if !r.IsIterable() {
		panic(fmt.Sprintf("flexbuffers object of type %s is not iterable", r.context.ItemVarType().toString()))
	}
	if !r.InsideBounds(item_index) {
		panic(fmt.Sprintf("out of bounds, index %d out of %d", item_index, r.item_count))
	}
	abs_offset := r.absItemOffset(item_index)
	bWidth := B(r.context.ItemByteSize())
	if !isInline(con.ItemVarType()) {
		abs_offset -= bytesAsUint(r.getBytes(abs_offset, bWidth)...)
	}
	r = Ref{buffer: r.buffer, index_0: abs_offset, context: con} //TODO: limit Ref access to flexbuffer
	r.item_count = r.itemCount()
	return r
}

func NewRef(buff []byte) *Ref {
	n := len(buff)
	bWidth := bytesAsUint(buff[n-1])
	con := context(bytesAsUint(buff[n-2]))
	index_0 := uint64(n - 2 - int(bWidth))
	var r *Ref
	if !isInline(con.ItemVarType()) {
		index_0 -= bytesAsUint(buff[index_0 : n-2]...)
	}
	r = &Ref{buff, con, index_0, 0}
	r.item_count = r.itemCount()
	return r
}

func (r Ref) InsideBounds(item_index int64) bool {
	return item_index < int64(r.item_count)
}

func (r Ref) getBytes(abs_offset uint64, bytes uint64) []byte {
	return r.buffer[abs_offset : abs_offset+bytes]
}

func (r Ref) relItemOffset(item_index int64) int64 {
	return int64(r.index_0 - uint64(item_index*int64(B(r.context.ItemByteSize()))))
}

func (r Ref) absItemOffset(item_index int64) uint64 {
	return r.absOffset(r.relItemOffset(item_index))
}

func (r Ref) absOffset(rel_offset int64) uint64 {
	i := r.index_0 - uint64(rel_offset)
	return i
}

func (r Ref) getItemContext(item_index int64) context {
	rel_offset := r.relItemOffset(item_index) + item_index
	abs_offset := r.absOffset(rel_offset)
	con := r.buffer[abs_offset]
	return context(con)
}

func (r Ref) itemCount() uint64 {
	switch r.context.ItemVarType() {
	case INT, UINT, FLOAT, BOOL, NULL, INDIRECT_UINT, INDIRECT_INT, INDIRECT_FLOAT:
		return 1
	case VECTOR_INT2, VECTOR_UINT2, VECTOR_FLOAT2:
		return 2
	case VECTOR_INT3, VECTOR_UINT3, VECTOR_FLOAT3:
		return 3
	case VECTOR_INT4, VECTOR_UINT4, VECTOR_FLOAT4:
		return 4
	case VECTOR, MAP, BLOB, STRING:
		rel_offset := r.relItemOffset(-1)
		abs_offset := r.absOffset(rel_offset)
		size := r.buffer[abs_offset]
		return uint64(size)
	case KEY:
		return uint64(bytes.IndexByte(r.buffer[r.index_0:], 0) + 1)
	}
	panic("an unexpected error has occured")
}

func (r Ref) IsUint() bool {
	return r.context.ItemVarType() == UINT
}

func bytesAsUint(bytes ...byte) uint64 {
	n := len(bytes)
	switch true {
	case n == 1:
		return uint64(bytes[0])
	case n == 2:
		return uint64(binary.LittleEndian.Uint16(bytes))
	case n == 4:
		return uint64(binary.LittleEndian.Uint32(bytes))
	case n == 8:
		return uint64(binary.LittleEndian.Uint64(bytes))
	default:
		panic(fmt.Sprintf("non-standard byte slice of length %d", n))
	}
	/*b8 := make([]byte, 8)
	copy(b8, bytes)
	u := binary.LittleEndian.Uint64(b8)
	return u*/
}

func (r Ref) Uint() (uint64, error) {
	if !r.IsUint() {
		return 0, fmt.Errorf("flexbuffers object of type %s can not be converted to uint", r.context.ItemVarType().toString())
	}

	bWidth := B(r.context.ItemByteSize())
	abs_offset := r.index_0
	return bytesAsUint(r.getBytes(abs_offset, bWidth)...), nil
}

func (r Ref) IsInt() bool {
	return r.context.ItemVarType() == INT
}

func bytesAsInt(bytes ...byte) int64 {
	u := bytesAsUint(bytes...)
	var i int64
	n := len(bytes)
	switch true {
	case n == 1:
		i = int64(int8(u))
	case n == 2:
		i = int64(int16(u))
	case n == 4:
		i = int64(int32(u))
	case n == 8:
		i = int64(u)
	default:
		panic(fmt.Sprintf("non-standard byte slice of length %d", n))
	}
	return i
}

func (r Ref) Int() (int64, error) {
	if !r.IsInt() {
		return 0, fmt.Errorf("flexbuffers object of type %s can not be converted to int", r.context.ItemVarType().toString())
	}
	abs_offset := r.index_0
	bWidth := B(r.context.ItemByteSize())
	return bytesAsInt(r.getBytes(abs_offset, bWidth)...), nil
}

func (r Ref) IsBool() bool {
	return r.context.ItemVarType() == BOOL
}

func bytesAsBool(bytes ...byte) bool {
	return bytes[0] > 0
}

func (r Ref) Bool() (bool, error) {
	if !r.IsBool() {
		return false, fmt.Errorf("flexbuffers object of type %s can not be converted to bool", r.context.ItemVarType().toString())
	}
	return bytesAsBool(r.buffer[r.index_0]), nil
}

func (r Ref) IsFloat() bool {
	return r.context.ItemVarType() == FLOAT
}

//assume slice size equals var size
/*
func bytesAsFloat(bytes ...byte) float64 {

}
*/
func (r Ref) Float() (float64, error) {
	if !r.IsFloat() {
		return 0, fmt.Errorf("flexbuffers object of type %s can not be converted to float", r.context.ItemVarType().toString())
	}
	abs_offset := r.index_0
	switch r.context.ItemByteSize() {
	case b32:
		x := r.buffer[abs_offset : abs_offset+B(b32)]
		u := binary.LittleEndian.Uint32(x)
		return float64(math.Float32frombits(u)), nil
	case b64:
		u := binary.LittleEndian.Uint64(r.buffer[abs_offset : abs_offset+B(b64)])
		return math.Float64frombits(u), nil
	}
	panic("an unexpected error has occured")
}

func (r Ref) IsNull() bool {
	return r.context.ItemVarType() == NULL
}

func (r Ref) IsString() bool {
	return r.context.ItemVarType() == STRING
}

func (r Ref) IsKey() bool {
	return r.context.ItemVarType() == KEY
}

func (r Ref) AsString() string {
	vType := r.context.ItemVarType()
	if vType == STRING || vType == KEY {
		return string(r.buffer[r.index_0 : r.index_0+uint64(r.item_count)])
	}
	//TODO: possibly add support for other types
	panic(fmt.Sprintf("type %s can not be expressed as string", vType.toString()))
}

func (r Ref) IsUntypedVector() bool {
	return r.context.ItemVarType() == VECTOR
}

func (r Ref) UntypedVector() ([]interface{}, error) {
	if !r.IsUntypedVector() {
		return nil, fmt.Errorf("flexbuffers object of type %s can not be converted to []interface{}", r.context.ItemVarType().toString())
	}
	result := []interface{}{}
	for i := int64(0); i < int64(r.item_count); i++ {
		item_context := r.getItemContext(i)
		item_ref := r.getItemAsRef(i, item_context)
		x, _ := item_ref.Interface()
		result = append(result, x)
	}
	return result, nil
}

func (r Ref) IsIntTyped() bool {
	return isIntTyped(r.context.ItemVarType())
}

func (r Ref) IntSlice() ([]int64, error) {
	if !r.IsIntTyped() {
		return nil, fmt.Errorf("flexbuffers object of type %s can not be converted to []int64", r.context.ItemVarType().toString())
	}
	bSize := r.context.ItemByteSize()
	result := []int64{}
	for i := int64(0); i < int64(r.item_count); i++ {
		item_ref := r.getItemAsRef(i, Pack(INT, bSize))
		x, _ := item_ref.Int() //HELP: can I just ignore unexpected errors like this one?
		result = append(result, x)
	}
	return result, nil
}

func (r Ref) IsUintTyped() bool {
	return isUintTyped(r.context.ItemVarType())
}

func (r Ref) UintSlice() ([]uint64, error) {
	if !r.IsUintTyped() {
		return nil, fmt.Errorf("flexbuffers object of type %s can not be converted to []uint64", r.context.ItemVarType().toString())
	}
	bSize := r.context.ItemByteSize()
	result := []uint64{}
	for i := int64(0); i < int64(r.item_count); i++ {
		item_ref := r.getItemAsRef(i, Pack(UINT, bSize))
		x, _ := item_ref.Uint()
		result = append(result, x)
	}
	return result, nil
}

func (r Ref) IsFloatTyped() bool {
	return isFloatTyped(r.context.ItemVarType())
}

func (r Ref) FloatSlice() ([]float64, error) {
	if !r.IsFloatTyped() {
		return nil, fmt.Errorf("flexbuffers object of type %s can not be converted to []float64", r.context.ItemVarType().toString())
	}
	bSize := r.context.ItemByteSize()
	result := []float64{}
	for i := int64(0); i < int64(r.item_count); i++ {
		item_ref := r.getItemAsRef(i, Pack(FLOAT, bSize))
		x, err := item_ref.Float()
		if err != nil {
			return nil, err
		}
		result = append(result, x)
	}
	return result, nil
}

func (r Ref) IsBoolTyped() bool {
	return isBoolTyped(r.context.ItemVarType())
}

func (r Ref) BoolSlice() ([]bool, error) {
	if !r.IsBoolTyped() {
		return nil, fmt.Errorf("flexbuffers object of type %s can not be converted to []bool", r.context.ItemVarType().toString())
	}
	result := []bool{}
	for i := int64(0); i < int64(r.item_count); i++ {
		item_ref := r.getItemAsRef(i, Pack(BOOL, b8))
		x, _ := item_ref.Bool()
		result = append(result, x)
	}
	return result, nil
}

func (r Ref) IsTyped() bool {
	return isTyped(r.context.ItemVarType())
}

func (r Ref) IsTypedVector() bool {
	return isTypedVector(r.context.ItemVarType())
}

func (r Ref) IsFixedTypedVector() bool {
	return isFixedTypedVector(r.context.ItemVarType())
}

func (r Ref) IsVector() bool {
	return isVector(r.context.ItemVarType())
}

func (r Ref) IsBlobLike() bool {
	return isBlobLike(r.context.ItemVarType())
}

func (r Ref) IsIterable() bool {
	return r.IsVector() || r.IsMap() || r.IsBlobLike()
}

func (r Ref) Index(i int64) (Ref, error) {
	vType := r.context.ItemVarType()
	if !r.IsIterable() {
		return Ref{}, fmt.Errorf("flexbuffers object of type %s does not support indexing", vType.toString())
	}
	if !r.InsideBounds(i) {
		return Ref{}, fmt.Errorf("out of bounds")
	}
	rel_offset := r.relItemOffset(i)
	abs_offset := r.absOffset(rel_offset)
	var context context
	if r.IsTyped() {
		switch true {
		case isUintTyped(vType):
			context = Pack(UINT, r.context.ItemByteSize())
		case isIntTyped(vType):
			context = Pack(INT, r.context.ItemByteSize())
		case isFloatTyped(vType):
			context = Pack(FLOAT, r.context.ItemByteSize())
		case isBoolTyped(vType):
			context = Pack(BOOL, r.context.ItemByteSize())
		case vType == VECTOR_KEY:
			context = Pack(KEY, b8)
		default:
			panic("an unknown error has occured")
		}

	} else {
		//untyped iterable
		context = r.getItemContext(i)
	}
	return r.getItemAsRef(int64(abs_offset), context), nil //HELP: double checking IsIterable,InsideBounds
}

func (r Ref) IsMap() bool {
	return r.context.ItemVarType() == MAP
}

func (r Ref) MapIndex(key string) (Ref, error) {
	if !r.IsMap() {
		return Ref{}, fmt.Errorf("flexbuffers object of type %s does not support key mapping", r.context.ItemVarType().toString())
	}
	key_vector_ref := r.KeyVector()
	var binary_search_keys func(string, Ref, int, int) int
	binary_search_keys = func(key string, kv_ref Ref, lower_bounds int, upper_bounds int) int {
		if lower_bounds >= upper_bounds {
			return -1 //nothing found
		}
		pivot := (upper_bounds - lower_bounds) / 2
		key_ref := kv_ref.getItemAsRef(int64(pivot), Pack(KEY, b8))
		key_bytes := key_ref.getBytes(key_ref.index_0, key_ref.item_count)
		c := bytes.Compare(key_bytes, *(*[]byte)(unsafe.Pointer(&key)))
		if c == 0 {
			return pivot
		}
		if c < 0 {
			return binary_search_keys(key, kv_ref, lower_bounds, pivot)
		}
		return binary_search_keys(key, kv_ref, pivot+1, upper_bounds)
	}
	item_index := binary_search_keys(key, key_vector_ref, 0, int(key_vector_ref.item_count))
	if item_index < 0 { //not found
		return Ref{}, fmt.Errorf("key not found in map")
	}
	return r.Index(int64(item_index))
}

func (r Ref) KeyVector() Ref {
	if !r.IsMap() {
		panic(fmt.Sprintf("flexbuffers object of type %s does not support key mapping", r.context.ItemVarType().toString()))
	}
	key_vector_bWidth := bytesAsUint(r.getBytes(r.index_0-2, 1)...)
	key_vector_offset := bytesAsUint(r.getBytes(r.index_0-3, 1)...)
	key_vector_index_0 := r.index_0 - key_vector_offset
	key_vector_context := Pack(VECTOR_KEY, b(int(key_vector_bWidth)))
	key_vector_ref := Ref{buffer: r.buffer, index_0: key_vector_index_0, context: key_vector_context}
	key_vector_ref.item_count = key_vector_ref.itemCount()
	return key_vector_ref
}

func (r Ref) Map() (map[string]interface{}, error) {
	if !r.IsMap() {
		return nil, fmt.Errorf("flexbuffers object of type %s does not support key mapping", r.context.ItemVarType().toString())
	}
	key_vector_ref := r.KeyVector()
	kv_bSize := b(int(key_vector_ref.context.ItemByteSize()))
	m := map[string]interface{}{}
	for i := int64(0); i < int64(r.item_count); i++ {
		key_ref := key_vector_ref.getItemAsRef(i, Pack(KEY, kv_bSize))
		k := key_ref.AsString()
		val_ref, err := r.MapIndex(k)
		if err != nil {
			return nil, err
		}
		v, err := val_ref.Interface()
		if err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, nil
}

func (r Ref) Interface() (interface{}, error) {
	if res, err := r.Int(); err == nil {
		return res, nil
	}
	if res, err := r.Uint(); err == nil {
		return res, nil
	}
	if res, err := r.Float(); err == nil {
		return res, nil
	}
	if res, err := r.Bool(); err == nil {
		return res, nil
	}
	if r.IsNull() {
		return nil, nil
	}
	if res, err := r.IntSlice(); err == nil {
		return res, nil
	}
	if res, err := r.UintSlice(); err == nil {
		return res, nil
	}
	if res, err := r.FloatSlice(); err == nil {
		return res, nil
	}
	if res, err := r.BoolSlice(); err == nil {
		return res, nil
	}
	if res, err := r.UntypedVector(); err == nil {
		return res, nil
	}
	if res, err := r.Map(); err == nil {
		return res, nil
	}
	return nil, fmt.Errorf("unexpected error - flexbuffer is corrupted. Unable to deserialize object of type %s", r.context.ItemVarType().toString())

}

//HELP: I question the need for this
type scanner struct {
	Ref
	index int64
}

func (s scanner) Next() bool {
	return s.InsideBounds(s.index)
}

func (s *scanner) Value() Ref {
	if !s.Next() {
		panic("scanner out of bounds")
	}
	r := s.getItemAsRef(s.index, s.context)
	s.index++
	return r
}

type VecScanner struct {
	scanner
}

func (r Ref) VecScan() (VecScanner, bool) {
	if !r.IsVector() {
		return VecScanner{}, false //HELP: shouldn't this be an error?
	}
	return VecScanner{scanner{r, 0}}, true
}

func (vs VecScanner) Index() int64 { //HELP: what's the reason this method should be exclusive to VecScanner?
	return vs.index
}

type MapScanner struct {
	scanner
	keyVec Ref
}

func (r Ref) MapScan() (MapScanner, bool) {
	if !r.IsMap() {
		return MapScanner{}, false //HELP: shouldn't this be an error?
	}
	kv := r.KeyVector()
	return MapScanner{scanner{r, 0}, kv}, true
}

func (ms MapScanner) Key() string {
	key, err := ms.keyVec.Index(ms.index)
	if err != nil {
		panic(err) //should never happen
	}
	return key.AsString()
}

/*
func (Ref) IsInt() bool
func (Ref) Int() (int, bool)
func (Ref) IsUint() bool
func (Ref) Uint() (int, bool)
func (Ref) IsT() bool    // {Float,Bool,String,Blob,Vector,Map,Null}
func (Ref) T() (T, bool) // {Float,Bool,String,Blob,Vector,Map}

// NB: in TS, these two are combined in a single get(key: number) function (that has a type violation?)
func (Ref) Index(i int) (Ref, bool)
func (Ref) MapIndex(k string) (Ref, bool)

func (Ref) Interface() (interface{}, error) // unpacks to a native {*int*,float*,bool,string,map[string]*,[]*,nil}

func (Ref) Len() int // XXX: unclear semantics (e.g. 1 if a map?)

func (Ref) Scan() (Scanner, bool)
func (Ref) MapScan() (MapScanner, bool)

type Scanner struct {
	buf []byte
}

func (Scanner) Next() bool
func (Scanner) Index() int // only if vec!
func (Scanner) Value() Ref

type MapScanner struct {
	buf []byte
}

func (MapScanner) Key() string // only if map!
func (MapScanner) Next() bool
func (MapScanner) Value() Ref

root := Reference(buf)
scanner, _ := root.Scan()

for scanner.Scan() {
	v := scanner.Value()
	i := scanner.Index()
}




r := Reference(buff)
arr :=[]int{}
for i:= 0;i<r.Len();i++ {
	ri,ok:=r.Index(i)
	if !ok {
		return error
	}
	n, err := ri.Float()
	if err != nil {
		return err
	}
	arr = append(arr,n)
}

func Unpack(buf []byte) interface{} {}
v,ok:= VectorInt(Unpack(buff))
if !ok {
	return error
}
arr :=[]int{}
for i:=0;i<v.itemCount();i++ {
	arr = append(arr,v.getItem(i))
}


var buf []byte
err := quick.Walk(buf, func(root quick.Ref) {
	n := root.Index(3).Int()
})
*/
