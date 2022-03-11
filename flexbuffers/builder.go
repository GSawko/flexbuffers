package flexbuffers

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Builder struct {
	finished       bool
	inProgress     []iStructure
	inProgressInit [4]iStructure
	headIndex      int
	root
}

//General API
func NewBuilder() *Builder {
	b := new(Builder)
	//b.root = Root{}
	b.inProgress = b.inProgressInit[:0]
	b.inProgress = append(b.inProgress, &b.root)
	return b
}

func (b *Builder) SerializeBuffer(buff *[]byte) (int, error) {
	return serialize(&b.root, buff, false)
}

func (b *Builder) Finish() error {
	if b.finished {
		return nil
	}
	if len(b.inProgress) == 1 {
		b.finished = true
		return nil
	}
	return fmt.Errorf("the builder has not yet finished: There are still objects waiting for construction")
}

func (b *Builder) IsFinished() bool {
	return b.finished
}

func (b *Builder) End() {
	if b.headIndex < 1 {
		panic("No structure to end")
	}
	b.inProgress = b.inProgress[:b.headIndex]
	b.headIndex--
	if b.headIndex == 0 {
		b.Finish()
	}
}

//internal API

func (b *Builder) getHead() iStructure {
	return b.inProgress[b.headIndex]
}

func (b *Builder) start(o iStructure) error {
	_, err := addOffsetToObject(b.getHead(), o)
	if err != nil {
		return err
	}

	b.headIndex++
	b.inProgress = append(b.inProgress, o)
	return nil
}

func (b *Builder) startWithKey(k *key, o iStructure) error {
	head := b.getHead()
	m, ok := head.(*flexMap)
	if k == nil {
		return fmt.Errorf("key must not be nil")
	}
	if !ok {
		return fmt.Errorf("type %T does not support key mapping", head)
	}
	if k.toString() == "" {
		return fmt.Errorf("mapping with empty key not allowed")
	}
	_, err := m.addOffsetWithKey(k, o)
	if err != nil {
		return err
	}
	b.headIndex++
	b.inProgress = append(b.inProgress, o)
	return nil
}

func (b *Builder) startWithOptionalKey(k *key, o iStructure) error {
	if k == nil {
		return b.start(o)
	}
	return b.startWithKey(k, o)
}

func (b *Builder) registerElement(elem element) error {
	head := b.getHead()
	_, err := addElement(head, elem)
	if err != nil {
		return err
	}
	if len(b.inProgress) == 1 {
		b.Finish()
	}
	return nil
}

func (b *Builder) registerElementWithKey(k *key, elem element) error {
	head := b.getHead()
	m, ok := head.(*flexMap)
	if k == nil {
		return fmt.Errorf("key must not be nil")
	}
	if !ok {
		return fmt.Errorf("type %T does not support key mapping", head)
	}
	if k.toString() == "" {
		return fmt.Errorf("mapping with empty key not allowed")
	}
	_, err := m.addElementWithKey(k, elem)
	return err
}

func (b *Builder) registerElementWithOptionalKey(k *key, elem element) error {
	if k == nil {
		return b.registerElement(elem)
	}
	return b.registerElementWithKey(k, elem)
}

func (b *Builder) parse(str string) (e error) { //parse string as a method call, mostly used for quickly building test cases
	e = nil
	_int := func(str string) int64 {
		if i, err := strconv.ParseInt(str, 10, 64); err == nil {
			return i
		}
		panic(fmt.Sprintf("could not parse '%s' to int64", str))
	}
	_uint := func(str string) uint64 {
		if u, err := strconv.ParseUint(str, 10, 64); err == nil {
			return u
		}
		panic(fmt.Sprintf("could not parse '%s' to uint64", str))
	}
	_float := func(str string) float64 {
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return f
		}
		panic(fmt.Sprintf("could not parse '%s' to float64", str))
	}
	_bool := func(str string) bool {
		if i, err := strconv.ParseBool(str); err == nil {
			return i
		}
		panic(fmt.Sprintf("could not parse '%s' to bool", str))
	}
	_string := func(str string) string { return str }
	vector := func(baseType VarType, maxcapacity uint64) (iStructure, error) { //return typed vector if maxcapacity=0 or fixed typed vector otherwise
		switch baseType {
		case INT, UINT, FLOAT:
			switch maxcapacity {
			case 0:
				return newTypedVector(baseType + 10), nil
			case 1:
				return newFixedTypedVector(baseType + 5), nil //Indirect
			case 2:
				return newFixedTypedVector(baseType + 15), nil //Tuple
			case 3:
				return newFixedTypedVector(baseType + 18), nil //Triple
			case 4:
				return newFixedTypedVector(baseType + 21), nil //Quad
			default:
				return nil, fmt.Errorf("fixed vector types of %d capacity not supported", maxcapacity)
			}
		case BOOL, STRING, KEY:
			if maxcapacity > 0 {
				return nil, fmt.Errorf("fixed vector types of base type %s not supported", baseType.toString())
			}
			switch baseType {
			case BOOL:
				return newTypedVector(VECTOR_BOOL), nil
			case STRING:
				return newTypedVector(VECTOR_STRING_DEPRECATED), nil
			case KEY:
				return newTypedVector(VECTOR_KEY), nil
			}
		default:
			return nil, fmt.Errorf("vector types of base type %s not supported", baseType.toString())
		}
		panic("unreachable")
	}
	reg := regexp.MustCompile(`(?:<(?P<key>[^>]+)>)?(?P<method>[a-zA-Z]+)\((?P<args>[^)]+)?\)`)
	var matchAndParse func(string, *regexp.Regexp) error
	matchAndParse = func(s string, r *regexp.Regexp) error {
		matches := r.FindStringSubmatch(s)
		if len(matches) == 0 {
			return nil
		}
		m := make(map[string]string)
		for _, name := range r.SubexpNames() {
			index := r.SubexpIndex(name)
			if index > 0 {
				str, err := strconv.Unquote(matches[index])
				if err != nil {
					m[name] = matches[index]
				} else {
					m[name] = str
				}
			}
		}
		args := []string{}
		if m["args"] != "" {
			args = strings.Split(m["args"], ",")
		}
		argcount := len(args)
		requireArgCount := func(count int) error {
			if count != argcount {
				return fmt.Errorf("expected %d argument(s), got %d", count, argcount)
			}
			return nil
		}
		defer func() {
			if r := recover(); r != nil {
				e = r.(error)
			}
		}()
		switch m["method"] {
		case "UINT":
			if err := requireArgCount(1); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.registerElementWithOptionalKey(k, newUINT(_uint(args[0])))
		case "INT":
			if err := requireArgCount(1); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.registerElementWithOptionalKey(k, newINT(_int(args[0])))
		case "FLOAT":
			if err := requireArgCount(1); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.registerElementWithOptionalKey(k, newFLOAT(_float(args[0])))
		case "BOOL":
			if err := requireArgCount(1); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.registerElementWithOptionalKey(k, newBOOL(_bool(args[0])))
		case "NULL":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.registerElementWithOptionalKey(k, newNULL())
		case "KEY":
			if err := requireArgCount(1); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newKey(_string(args[0])))
			b.End()
		case "STRING":
			if err := requireArgCount(1); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newFlexString(_string(args[0])))
			b.End()
		case "VEC":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newVector())
		case "INTVEC":
			if err := requireArgCount(1); err != nil {
				return err
			}
			v, err := vector(INT, _uint(args[0]))
			if err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, v)
		case "UINTVEC":
			if err := requireArgCount(1); err != nil {
				return err
			}
			v, err := vector(UINT, _uint(args[0]))
			if err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, v)
		case "FLOATVEC":
			if err := requireArgCount(1); err != nil {
				return err
			}
			v, err := vector(FLOAT, _uint(args[0]))
			if err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, v)
		case "BOOLVEC":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newTypedVector(VECTOR_BOOL))
		case "BLOB":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			bytes := []byte{}
			for _, arg := range args {
				bytes = append(bytes, byte(_uint(arg)))
			}
			e = b.startWithOptionalKey(k, newBlob(bytes))
			b.End()
		case "STRINGVEC":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newTypedVector(VECTOR_STRING_DEPRECATED))
		case "KEYVEC":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newKeyVector())
		case "MAP":
			if err := requireArgCount(0); err != nil {
				return err
			}
			var k *key = nil
			if m["key"] != "" {
				k = newKey(m["key"])
			}
			e = b.startWithOptionalKey(k, newFlexMap())
		case "END":
			if err := requireArgCount(0); err != nil {
				return err
			}
			b.End()
		}
		if e != nil {
			return e
		}
		if len(matches[0])+1 > len(s) {
			return nil
		}
		return matchAndParse(s[len(matches[0])+1:], r)
	}
	return matchAndParse(str, reg)
}

//API for basic types

func (b *Builder) Int(i int64) error {
	return b.registerElement(newINT(i))
}
func (b *Builder) Uint(u uint64) error {
	return b.registerElement(newUINT(u))
}
func (b *Builder) Float(f float64) error {
	return b.registerElement(newFLOAT(f))
}
func (b *Builder) Bool(l bool) error {
	return b.registerElement(newBOOL(l))
}
func (b *Builder) Null() error {
	return b.registerElement(newNULL())
}

//API for vectors & typed vectors

func (b *Builder) StartVector() error {
	return b.start(newVector())
}

func (b *Builder) StartTypedIntVector() error {
	return b.start(newTypedVector(VECTOR_INT))
}

func (b *Builder) StartTypedUintVector() error {
	return b.start(newTypedVector(VECTOR_UINT))
}

func (b *Builder) StartTypedFloatVector() error {
	return b.start(newTypedVector(VECTOR_FLOAT))
}

func (b *Builder) StartTypedBoolVector() error {
	return b.start(newTypedVector(VECTOR_BOOL))
}

func (b *Builder) StartBlob(bytes []byte) error {
	return b.start(newBlob(bytes))
}

//API for scalars, tuples, triples, quads

func (b *Builder) StartIntScalar() error {
	return b.start(newFixedTypedVector(INDIRECT_INT))
}

func (b *Builder) StartUintScalar() error {
	return b.start(newFixedTypedVector(INDIRECT_UINT))
}

func (b *Builder) StartFloatScalar() error {
	return b.start(newFixedTypedVector(INDIRECT_FLOAT))
}

func (b *Builder) StartIntTuple() error {
	return b.start(newFixedTypedVector(VECTOR_INT2))
}

func (b *Builder) StartUintTuple() error {
	return b.start(newFixedTypedVector(VECTOR_UINT2))
}

func (b *Builder) StartFloatTuple() error {
	return b.start(newFixedTypedVector(VECTOR_FLOAT2))
}

func (b *Builder) StartIntTriple() error {
	return b.start(newFixedTypedVector(VECTOR_INT3))
}

func (b *Builder) StartUintTriple() error {
	return b.start(newFixedTypedVector(VECTOR_UINT3))
}

func (b *Builder) StartFloatTriple() error {
	return b.start(newFixedTypedVector(VECTOR_FLOAT3))
}

func (b *Builder) StartIntQuad() error {
	return b.start(newFixedTypedVector(VECTOR_INT4))
}

func (b *Builder) StartUintQuad() error {
	return b.start(newFixedTypedVector(VECTOR_UINT4))
}

func (b *Builder) StartFloatQuad() error {
	return b.start(newFixedTypedVector(VECTOR_FLOAT4))
}

//API for string types

func (b *Builder) StartString() error {
	return b.start(newFlexString(""))
}

func (b *Builder) String(str string) error {
	defer b.End()
	return b.start(newFlexString(str))
}

func (b *Builder) StartKey() error {
	return b.start(newKey(""))
}

func (b *Builder) Key(str string) error {
	defer b.End()
	return b.start(newKey(str))
}

func (b *Builder) Append(str string) error { //maybe other vector types should also support appending?
	head := b.getHead()
	s, ok := head.(*flexString)
	if !ok {
		return fmt.Errorf("type %T is not a string-type and does not support appending", head)
	}
	s.Append(str)
	return nil
}

//API for maps

func (b *Builder) StartMap() error {
	return b.start(newFlexMap())
}

func (b *Builder) StartMapWithKey(k string) error {
	return b.startWithKey(newKey(k), newFlexMap())
}

func (b *Builder) UintWithKey(k string, u uint64) error {
	return b.registerElementWithKey(newKey(k), newUINT(u))
}

func (b *Builder) IntWithKey(k string, i int64) error {
	return b.registerElementWithKey(newKey(k), newINT(i))
}

func (b *Builder) FloatWithKey(k string, f float64) error {
	return b.registerElementWithKey(newKey(k), newFLOAT(f))
}

func (b *Builder) BoolWithKey(k string, l bool) error {
	return b.registerElementWithKey(newKey(k), newBOOL(l))
}

func (b *Builder) NullWithKey(k string) error {
	return b.registerElementWithKey(newKey(k), newNULL())
}

func (b *Builder) StartVectorWithKey(k string) error {
	return b.startWithKey(newKey(k), newVector())
}

func (b *Builder) StartTypedIntVectorWithKey(k string) error {
	return b.startWithKey(newKey(k), newTypedVector(VECTOR_INT))
}

func (b *Builder) StartTypedUintVectorWithKey(k string) error {
	return b.startWithKey(newKey(k), newTypedVector(VECTOR_UINT))
}

func (b *Builder) StartTypedFloatVectorWithKey(k string) error {
	return b.startWithKey(newKey(k), newTypedVector(VECTOR_FLOAT))
}

func (b *Builder) StartTypedBoolVectorWithKey(k string) error {
	return b.startWithKey(newKey(k), newTypedVector(VECTOR_BOOL))
}

func (b *Builder) StartBlobWithKey(k string, bytes []byte) error {
	return b.startWithKey(newKey(k), newBlob(bytes))
}

//API for scalars, tuples, triples, quads

func (b *Builder) StartIntScalarWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(INDIRECT_INT))
}

func (b *Builder) StartUintScalarWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(INDIRECT_UINT))
}

func (b *Builder) StartFloatScalarWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(INDIRECT_FLOAT))
}

func (b *Builder) StartIntTupleWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_INT2))
}

func (b *Builder) StartUintTupleWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_UINT2))
}

func (b *Builder) StartFloatTupleWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_FLOAT2))
}

func (b *Builder) StartIntTripleWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_INT3))
}

func (b *Builder) StartUintTripleWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_UINT3))
}

func (b *Builder) StartFloatTripleWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_FLOAT3))
}

func (b *Builder) StartIntQuadWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_INT4))
}

func (b *Builder) StartUintQuadWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_UINT4))
}

func (b *Builder) StartFloatQuadWithKey(k string) error {
	return b.startWithKey(newKey(k), newFixedTypedVector(VECTOR_FLOAT4))
}

func (b *Builder) StartStringWithKey(k string) error {
	return b.startWithKey(newKey(k), newFlexString(""))
}

func (b *Builder) StringWithKey(k string, str string) error {
	defer b.End()
	return b.startWithKey(newKey(k), newFlexString(str))
}

//Kinda silly

// func (b *Builder) StartKeyWithKey(k string) error {
// 	return b.startWithKey(newKey(k), newKey(""))
// }

// func (b *Builder) KeyWithKey(k string, str string) error {
// 	defer b.End()
// 	return b.startWithKey(newKey(k), newKey(str))
// }

//Auto-Building

func (b *Builder) AutoBuild(item interface{}) error {
	var auto func(*key, interface{}) error

	auto = func(k *key, item interface{}) error {
		slicetype := false
		switch x := item.(type) {
		case uint:
			return b.registerElementWithOptionalKey(k, newUINT(uint64(x)))
		case uint8:
			return b.registerElementWithOptionalKey(k, newUINT(uint64(x)))
		case uint16:
			return b.registerElementWithOptionalKey(k, newUINT(uint64(x)))
		case uint32:
			return b.registerElementWithOptionalKey(k, newUINT(uint64(x)))
		case uint64:
			return b.registerElementWithOptionalKey(k, newUINT(x))
		case int:
			return b.registerElementWithOptionalKey(k, newINT(int64(x)))
		case int8:
			return b.registerElementWithOptionalKey(k, newINT(int64(x)))
		case int16:
			return b.registerElementWithOptionalKey(k, newINT(int64(x)))
		case int32:
			return b.registerElementWithOptionalKey(k, newINT(int64(x)))
		case int64:
			return b.registerElementWithOptionalKey(k, newINT(x))
		case float32:
			return b.registerElementWithOptionalKey(k, newFLOAT(float64(x)))
		case float64:
			return b.registerElementWithOptionalKey(k, newFLOAT(x))
		case bool:
			return b.registerElementWithOptionalKey(k, newBOOL(x))
		case string:
			defer b.End()
			return b.startWithOptionalKey(k, newFlexString(x))
		case map[string]interface{}:
			defer b.End()
			b.startWithOptionalKey(k, newFlexMap())
			for k, v := range x {
				var K *key = nil
				if k != "" {
					K = newKey(k)
				}
				auto(K, v)
			}
		case []interface{}, []string:
			defer b.End()
			if err := b.startWithOptionalKey(k, newVector()); err != nil {
				return err
			}
			slicetype = true
		case []uint, []uint8, []uint16, []uint32, []uint64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newTypedVector(VECTOR_UINT)); err != nil {
				return err
			}
			slicetype = true
		case []int, []int8, []int16, []int32, []int64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newTypedVector(VECTOR_INT)); err != nil {
				return err
			}
			slicetype = true
		case []float32, []float64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newTypedVector(VECTOR_FLOAT)); err != nil {
				return err
			}
			slicetype = true
		case []bool:
			defer b.End()
			if err := b.startWithOptionalKey(k, newTypedVector(VECTOR_BOOL)); err != nil {
				return err
			}
			slicetype = true
		case [1]uint8, [1]uint16, [1]uint32, [1]uint64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(INDIRECT_UINT)); err != nil {
				return err
			}
			slicetype = true
		case [1]int8, [1]int16, [1]int32, [1]int64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(INDIRECT_INT)); err != nil {
				return err
			}
			slicetype = true
		case [1]float32, [1]float64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(INDIRECT_FLOAT)); err != nil {
				return err
			}
			slicetype = true
		case [2]uint8, [2]uint16, [2]uint32, [2]uint64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_UINT2)); err != nil {
				return err
			}
			slicetype = true
		case [2]int8, [2]int16, [2]int32, [2]int64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_INT2)); err != nil {
				return err
			}
			slicetype = true
		case [2]float32, [2]float64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_FLOAT2)); err != nil {
				return err
			}
			slicetype = true
		case [3]uint8, [3]uint16, [3]uint32, [3]uint64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_UINT3)); err != nil {
				return err
			}
			slicetype = true
		case [3]int8, [3]int16, [3]int32, [3]int64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_INT3)); err != nil {
				return err
			}
			slicetype = true
		case [3]float32, [3]float64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_FLOAT3)); err != nil {
				return err
			}
			slicetype = true
		case [4]uint8, [4]uint16, [4]uint32, [4]uint64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_UINT4)); err != nil {
				return err
			}
			slicetype = true
		case [4]int8, [4]int16, [4]int32, [4]int64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_INT4)); err != nil {
				return err
			}
			slicetype = true
		case [4]float32, [4]float64:
			defer b.End()
			if err := b.startWithOptionalKey(k, newFixedTypedVector(VECTOR_FLOAT4)); err != nil {
				return err
			}
			slicetype = true
		default:
			return fmt.Errorf("unable to auto-build type %T", x)
		}
		if slicetype {
			v := reflect.ValueOf(item)
			for i := 0; i < v.Len(); i++ {
				if err := auto(nil, v.Index(i).Interface()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return auto(nil, item)

}
