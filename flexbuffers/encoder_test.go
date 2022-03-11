package flexbuffers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

type TestData struct {
	testArgs       []interface{}
	expectedResult interface{}
}

type TestCase struct {
	data         []TestData
	testCall     func(*testing.T, ...interface{}) interface{}
	testVerifier func(result interface{}, expected interface{}) bool
	name         string
	verbose      bool
}

type dualString struct { //used for clarity
	a            string
	b            string
	sprintfitems []interface{}
}

func dString(a string, b string, sprintfitems ...interface{}) dualString {
	return dualString{a, b, sprintfitems}
}

func dualSprintf(a string, b string, sprintfitems ...interface{}) (string, string) {
	return fmt.Sprintf(a, sprintfitems...), fmt.Sprintf(b, sprintfitems...)
}

func (test *TestCase) Verify(t *testing.T) {
	ok := true
	failedTests := []int{}
	for batch, data := range test.data {
		result := test.testCall(t, data.testArgs...)
		if !test.testVerifier(result, data.expectedResult) {
			ok = false
			t.Logf("Test %s failed for data batch #%d - result: %T %#v, expected: %T %#v", test.name, batch+1, result, result, data.expectedResult, data.expectedResult)
			failedTests = append(failedTests, batch+1)
			continue
		}
		if test.verbose {
			t.Logf("%d: %v (%v)", batch+1, result, data.expectedResult)
		}

	}
	require.True(t, ok, fmt.Sprintf("Test failed for batches: %v", failedTests))
}

func (test *TestCase) MutateData(f func(t *TestData)) {
	for i := range test.data {
		f(&test.data[i])
	}
}

func NewTestCase() TestCase {
	return TestCase{testVerifier: func(result interface{}, expected interface{}) bool {
		return result == expected
	},
	}
}

func bytesEqual(result interface{}, expected interface{}) bool {
	b1, ok1 := result.([]byte)
	b2, ok2 := expected.([]byte)
	if !(ok1 && ok2) {
		panic("Wrong datatype: []byte expected")
	}
	if len(b1) != len(b2) {
		return false
	}
	for i := range b1 {
		if b1[i] != b2[i] {
			return false
		}
	}
	return true
}

func stringsEqual(result interface{}, expected interface{}) bool {
	str1, ok1 := result.(string)
	str2, ok2 := expected.(string)
	if !(ok1 && ok2) {
		panic("Wrong datatype: string expected")
	}
	return str1 == str2
}

func flatcEncode(str string) []byte {
	v := json.RawMessage(str)
	b, err := encodeA(v)
	if err != nil {
		panic(err)
	}
	return b
}

func flexevalEncode(str string) []byte {
	cmd := exec.Command("python3", "-m", "flexeval", str)
	//cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "PYTHONPATH=/home/gs/flexbuffers/python:/home/gs/flexbuffers/go/flexbuffers/testdata")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("str = %q err = %v out = %s", str, err, out))
	}
	return out
}

func test_serialize(t *testing.T, args ...interface{}) interface{} {
	assert.Assert(t, len(args) == 1, fmt.Sprintf("expected 1 argument, got %d", len(args)))
	str, ok := args[0].(string)
	assert.Assert(t, ok, fmt.Sprintf("expected string argument, got %T", args))
	builder := NewBuilder()
	err := builder.parse(str)
	assert.NilError(t, err, fmt.Sprintf("encountered an unexpected error while parsing test case: %s", err))
	var buff []byte
	_, err = builder.SerializeBuffer(&buff)
	require.NoError(t, err, "Serialization has failed")
	return buff
}

func test_deserialize(t *testing.T, args ...interface{}) interface{} {
	assert.Assert(t, len(args) == 1, fmt.Sprintf("expected 1 argument, got %d", len(args)))
	i := test_serialize(t, args...)
	bytes, ok := i.([]byte)
	assert.Assert(t, ok, fmt.Sprintf("expected []byte argument, got %T", args))
	r := NewRef(bytes)
	i, err := r.Interface()
	require.NoError(t, err, fmt.Sprintf("decoding failed %s", err))
	j, err := json.Marshal(i)
	require.NoError(t, err, "unable to convert the result to json")
	return string(j)
}

func test_flatc_decode(t *testing.T, args ...interface{}) interface{} {
	i := test_serialize(t, args...)
	buff, ok := i.([]byte)
	assert.Assert(t, ok, "Expected type []byte, got %T", i)
	j, err := flatcDecode(&buff)
	require.NoError(t, err, fmt.Sprintf("decoding failed %s", err))
	return string(j)
}

func TestInlineTypesFlexeval(t *testing.T) {
	strings := []string{
		/*		fmt.Sprintf("INT(0)"),
						fmt.Sprintf("INT(1)"),
						fmt.Sprintf("INT(%d)", 1<<7-1),
						fmt.Sprintf("INT(%d)", 1<<7),
						fmt.Sprintf("INT(%d)", 1<<15-1),
						fmt.Sprintf("INT(%d)", 1<<15),
						fmt.Sprintf("INT(%d)", 1<<31-1),
						fmt.Sprintf("INT(%d)", 1<<31),
						fmt.Sprintf("INT(%d)", 1<<63-1),
						fmt.Sprintf("INT(%d)", -(1 << 7)),
						fmt.Sprintf("INT(%d)", -(1<<7 + 1)),
						fmt.Sprintf("INT(%d)", -(1 << 15)),
						fmt.Sprintf("INT(%d)", -(1<<15 + 1)),
						fmt.Sprintf("INT(%d)", -(1 << 31)),
						fmt.Sprintf("INT(%d)", -(1<<31 + 1)),
						fmt.Sprintf("INT(%d)", -(1 << 63)),
				//Floats
				fmt.Sprintf("FLOAT(0.0)"),
				fmt.Sprintf("FLOAT(1.0)"),*/
		fmt.Sprintf("FLOAT(340282346638528859811704183484516925440.00000)"), //WTF FlexEval does not represent floats correctly
		fmt.Sprintf("FLOAT(%f)", -math.MaxFloat32),
		fmt.Sprintf("FLOAT(%f)", math.MaxFloat32+math.SmallestNonzeroFloat32),
		fmt.Sprintf("FLOAT(%f)", -(math.MaxFloat32)),
		fmt.Sprintf("FLOAT(%f)", math.MaxFloat64),
		fmt.Sprintf("FLOAT(%f)", -math.MaxFloat64+math.SmallestNonzeroFloat64),
		//BOOL
		fmt.Sprintf("BOOL(True)"),
		fmt.Sprintf("BOOL(False)"),
		//NULL
		fmt.Sprintf("NULL()"),
	}

	testElement := NewTestCase()
	testElement.name = "testElementEncoding"
	testElement.data = make([]TestData, len(strings))
	for i, str := range strings {
		testElement.data[i] = TestData{[]interface{}{str}, str}
	}
	testElement.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flexevalEncode(s)
	})
	testElement.testVerifier = bytesEqual
	testElement.testCall = test_serialize
	testElement.verbose = true
	t.Run(testElement.name, testElement.Verify)
	testElement.testVerifier = stringsEqual
	testElement.name = "testElementDecoding"
	testElement.MutateData(func(t *TestData) {
		b, ok := t.expectedResult.([]byte)
		if !ok {
			panic(fmt.Sprintf("Expected type []byte,got %T", t.expectedResult))
		}
		j, err := flatcDecode(&b)
		if err != nil {
			panic("Decoding failed")
		}
		t.expectedResult = string(j)
	})
	testElement.testCall = test_deserialize
	t.Run(testElement.name, testElement.Verify)
}

func TestInlineTypesFlatc(t *testing.T) {
	dStrings := []dualString{
		dString("INT(%d)", "%d", -(1 << 7)),
		dString("INT(%d)", "%d", -(1<<7 + 1)),
		dString("INT(%d)", "%d", -(1 << 15)),
		dString("INT(%d)", "%d", -(1<<15 + 1)),
		dString("INT(%d)", "%d", -(1 << 31)),
		dString("INT(%d)", "%d", -(1<<31 + 1)),
		dString("INT(%d)", "%d", -(1 << 63)),
		//Floats
		dString("FLOAT(0.0)", "0.0"),
		dString("FLOAT(1.0)", "1.0"),
		dString("FLOAT(%f)", "%f", math.MaxFloat32),
		dString("FLOAT(%f)", "%f", -math.MaxFloat32),
		dString("FLOAT(%f)", "%f", math.MaxFloat32+math.SmallestNonzeroFloat32),
		dString("FLOAT(%f)", "%f", -(math.MaxFloat32 + math.SmallestNonzeroFloat32)),
		dString("FLOAT(%f)", "%f", math.MaxFloat64),
		dString("FLOAT(%f)", "%f", -math.MaxFloat64),
		//BOOL
		dString("BOOL(true)", "true"),
		dString("BOOL(false)", "false"),
		//NULL
		dString("NULL()", "null"),
	}

	testElement := NewTestCase()
	testElement.name = "testElementEncoding"
	testElement.data = make([]TestData, len(dStrings))
	for i, dstr := range dStrings {
		a, b := dualSprintf(dstr.a, dstr.b, dstr.sprintfitems...)
		testElement.data[i] = TestData{[]interface{}{a}, b}
	}
	testElement.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flatcEncode(s)
	})
	testElement.testVerifier = bytesEqual
	testElement.testCall = test_serialize
	testElement.verbose = true
	t.Run(testElement.name, testElement.Verify)
	testElement.testVerifier = stringsEqual
	testElement.name = "testElementDecoding"
	testElement.MutateData(func(t *TestData) {
		b, ok := t.expectedResult.([]byte)
		if !ok {
			panic(fmt.Sprintf("Expected type []byte,got %T", t.expectedResult))
		}
		j, err := flatcDecode(&b)
		if err != nil {
			panic("Decoding failed")
		}
		t.expectedResult = string(j)
	})
	testElement.testCall = test_flatc_decode

	t.Run(testElement.name, testElement.Verify)
}

func TestVectorTypesFlexeval(t *testing.T) {
	strings := []string{
		fmt.Sprintf("VEC() END()"),
		fmt.Sprintf("VEC() INT(%d) END()", math.MaxInt8), //WTF: Flatc does not do padding where expected
		fmt.Sprintf("VEC() INT(%d) END()", math.MaxInt16),
		fmt.Sprintf("VEC() INT(%d) END()", math.MaxInt32),
		fmt.Sprintf("VEC() INT(%d) END()", math.MaxInt64),
		fmt.Sprintf("VEC() INT(10) INT(-10) BOOL(True) INT(%d) END()", math.MaxInt32),
		fmt.Sprintf("VEC() INT(10) VEC() INT(-10) BOOL(True) END() END()"),
		fmt.Sprintf("VEC() VEC() INT(-10) BOOL(True) END() END()"),
		fmt.Sprintf("VEC() INT(-10) BOOL(True) END()"),
		fmt.Sprintf("VEC() VEC() INT(%d) END() END()", math.MaxInt32),
		fmt.Sprintf("VEC() VEC() INT(123) END() VEC() INT(321) END() END()"),
		fmt.Sprintf("VEC() VEC() INT(123) END() VEC() INT(%d) END() END()", math.MaxInt32),
		fmt.Sprintf("VEC() INT(10) VEC() INT(-10) BOOL(True) END() VEC() INT(%d) END() END()", math.MaxInt32),
	}
	testVector := NewTestCase()
	testVector.name = "testVectorEncoding"
	testVector.data = make([]TestData, len(strings))
	for i, str := range strings {
		testVector.data[i] = TestData{[]interface{}{str}, str}
	}
	testVector.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flexevalEncode(s)
	})
	testVector.testCall = test_serialize
	testVector.testVerifier = bytesEqual
	testVector.verbose = true
	t.Run(testVector.name, testVector.Verify)
	//decoding
	testVector.testVerifier = stringsEqual
	testVector.name = "testVectorDecoding"
	testVector.MutateData(func(t *TestData) {
		b, ok := t.expectedResult.([]byte)
		if !ok {
			panic(fmt.Sprintf("Expected type []byte,got %T", t.expectedResult))
		}
		j, err := flatcDecode(&b)
		if err != nil {
			panic("Decoding failed")
		}
		t.expectedResult = string(j)
	})
	testVector.testCall = test_flatc_decode
	t.Run(testVector.name, testVector.Verify)
}

func TestVectorTypesFlatc(t *testing.T) {
	dStrings := []dualString{
		dString("VEC() END()", "[]"),
		dString("VEC() INT(%d) END()", "[%d]", math.MaxInt8), //WTF: Flatc does not do padding where expected
		dString("VEC() INT(%d) END()", "[%d]", math.MaxInt16),
		dString("VEC() INT(%d) END()", "[%d]", math.MaxInt32),
		dString("VEC() INT(%d) END()", "[%d]", math.MaxInt64),
		dString("VEC() INT(10) INT(-10) BOOL(true) INT(%d) END()", "[10,-10,true,%d]", math.MaxInt32),
		dString("VEC() INT(10) VEC() INT(-10) BOOL(true) END() END()", "[10,[-10,true]]"),
		dString("VEC() VEC() INT(-10) BOOL(true) END() END()", "[[-10,true]]"),
		dString("VEC() INT(-10) BOOL(true) END()", "[-10,true]"),
		dString("VEC() VEC() INT(%d) END() END()", "[[%d]]", math.MaxInt32),
		dString("VEC() VEC() INT(123) END() VEC() INT(321) END() END()", "[[123],[321]]"),
		dString("VEC() VEC() INT(123) END() VEC() INT(%d) END() END()", "[[123],[%d]]", math.MaxInt32),
		dString("VEC() INT(10) VEC() INT(-10) BOOL(true) END() VEC() INT(%d) END() END()", "[10,[-10,true],[%d]]", math.MaxInt16),
	}
	testVector := NewTestCase()
	testVector.name = "testVectorEncoding"
	testVector.data = make([]TestData, len(dStrings))
	for i, dstr := range dStrings {
		a, b := dualSprintf(dstr.a, dstr.b, dstr.sprintfitems...)
		testVector.data[i] = TestData{[]interface{}{a}, b}
	}
	testVector.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flatcEncode(s)
	})
	testVector.testCall = test_serialize
	testVector.testVerifier = bytesEqual
	testVector.verbose = true
	t.Run(testVector.name, testVector.Verify)
	//decoding
	testVector.testVerifier = stringsEqual
	testVector.name = "testVectorDecoding"
	testVector.MutateData(func(t *TestData) {
		b, ok := t.expectedResult.([]byte)
		if !ok {
			panic(fmt.Sprintf("Expected type []byte,got %T", t.expectedResult))
		}
		j, err := flatcDecode(&b)
		if err != nil {
			panic("Decoding failed")
		}
		t.expectedResult = string(j)
	})
	testVector.testCall = test_flatc_decode
	t.Run(testVector.name, testVector.Verify)
}

func TestMapTypesFlexeval(t *testing.T) {
	strings := []string{
		fmt.Sprintf("MAP() END()"),
		fmt.Sprintf("MAP() <One>INT(1) END()"),
		fmt.Sprintf("MAP() <One>INT(1) <Two>INT(2) END()"),
		fmt.Sprintf("MAP() <One>INT(1) <Two>INT(2) <Three>INT(3) END()"),
		fmt.Sprintf("MAP() <Int8>INT(%d) END()", math.MaxInt8),
		fmt.Sprintf("MAP() <Int16>INT(%d) END()", math.MaxInt16),
		fmt.Sprintf("MAP() <Int32>INT(%d) END()", math.MaxInt32),
		fmt.Sprintf("MAP() <Int64>INT(%d) END()", math.MaxInt64),
		fmt.Sprintf("MAP() <Int8>INT(%d) <Int16>INT(%d) <Int32>INT(%d) <Int64>INT(%d) END()", math.MaxInt8, math.MaxInt16, math.MaxInt32, math.MaxInt64),
		fmt.Sprintf("MAP() <Xx>STRING(%s) <Yy>STRING(%s) END()", `"is x"`, `"is y"`),
		fmt.Sprintf("MAP() <A>STRING(Alpha) <B>STRING(BETA) <Y>STRING(GAMMA) END()"),
		fmt.Sprintf("MAP() <A>STRING(Alpha) <One>INT(1) <f025>FLOAT(0.25) END()"),
		fmt.Sprintf("MAP() <MyVec>VEC() INT(1) INT(2) INT(3) END() END()"),
		fmt.Sprintf("MAP() <MyVec>VEC() INT(1) INT(2) INT(3) END() <MyString>STRING(%s) END()", `"This is my string!"`),
		fmt.Sprintf("MAP() <MyVec>VEC() INT(1) INT(2) INT(%d) END() <OtherVec>VEC() INT(3) INT(4) FLOAT(%f) END() END()", math.MaxInt32, math.MaxFloat64),
		fmt.Sprintf("MAP() <one>INT(1) <MyMap>MAP() <f>FLOAT(%f) <MyString>STRING(%s) END() <i>INT(%d) END()", math.MaxFloat64, `"I ran out of ideas for difficult test cases..."`, math.MaxInt16),
	}
	testMap := NewTestCase()
	testMap.name = "testMapEncode"
	testMap.data = make([]TestData, len(strings))
	for i, str := range strings {
		testMap.data[i] = TestData{[]interface{}{str}, str}
	}
	testMap.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flexevalEncode(s)
	})
	testMap.testCall = test_serialize
	testMap.testVerifier = bytesEqual
	testMap.verbose = true
	t.Run(testMap.name, testMap.Verify)
	//Decoding
	testMap.MutateData(func(t *TestData) {
		b, ok := t.expectedResult.([]byte)
		if !ok {
			panic(fmt.Sprintf("Expected type []byte,got %T", t.expectedResult))
		}
		j, err := flatcDecode(&b)
		if err != nil {
			panic("Decoding failed")
		}
		t.expectedResult = string(j)
	})
	testMap.testCall = test_flatc_decode
	testMap.testVerifier = stringsEqual
	t.Run(testMap.name, testMap.Verify)
}

func TestMapTypesFlatc(t *testing.T) {
	dStrings := []dualString{
		dString("MAP() END()", "{}"),
		dString("MAP() <One>INT(1) END()", "{One:1}"),
		dString("MAP() <One>INT(1) <Two>INT(2)  END()", "{One:1, Two:2}"),
		dString("MAP() <One>INT(1) <Two>INT(2) <Three>INT(3) END()", `{"One":1, "Two":2, "Three":3}`),
		dString("MAP() <Int8>INT(%d) END()", "{Int8:%d}", math.MaxInt8),
		dString("MAP() <Int16>INT(%d) END()", "{Int16:%d}", math.MaxInt16),
		dString("MAP() <Int32>INT(%d) END()", "{Int32:%d}", math.MaxInt32),
		dString("MAP() <Int64>INT(%d) END()", "{Int64:%d}", math.MaxInt64),
		dString("MAP() <Int8>INT(%d) <Int16>INT(%d) <Int32>INT(%d) <Int64>INT(%d) END()", "{Int8:%d, Int16:%d, Int32:%d, Int64:%d}", math.MaxInt8, math.MaxInt16, math.MaxInt32, math.MaxInt64),
		dString("MAP() <Xx>STRING(%s) <Yy>STRING(%s) END()", `{"Xx":%s, "Yy":%s}`, `"is x"`, `"is y"`),
		dString("MAP() <A>STRING(Alpha) <B>STRING(BETA) <Y>STRING(GAMMA) END()", `{A:"Alpha", B:"BETA", Y:"GAMMA"}`),
		dString("MAP() <A>STRING(Alpha) <One>INT(1) <f025>FLOAT(0.25) END()", `{A:"Alpha", One:1, f025:0.25}`),
		dString("MAP() <MyVec>VEC() INT(1) INT(2) INT(3) END() END()", "{MyVec:[1,2,3]}"),
		dString("MAP() <MyVec>VEC() INT(1) INT(2) INT(3) END() <MyString>STRING(%s) END()", "{MyVec:[1,2,3], MyString:%s}", `"This is my string!"`),
		dString("MAP() <MyVec>VEC() INT(1) INT(2) INT(%d) END() <OtherVec>VEC() INT(3) INT(4) FLOAT(%f) END()", "{MyVec:[1,2,%d], OtherVec:[3,4,%f] }", math.MaxInt32, math.MaxFloat64),
		dString("MAP() <one>INT(1) <MyMap>MAP() <f>FLOAT(%f) <MyString>STRING(%s) END() <i>INT(%d) END()", "{one:1, MyMap:{f:%f, MyString:%s}, i:%d}", math.MaxFloat64, `"I ran out of ideas for difficult test cases..."`, math.MaxInt16),
	}
	testMap := NewTestCase()
	testMap.name = "testMapEncode"
	testMap.data = make([]TestData, len(dStrings))
	for i, dstr := range dStrings {
		a, b := dualSprintf(dstr.a, dstr.b, dstr.sprintfitems...)
		testMap.data[i] = TestData{[]interface{}{a}, b}
	}
	testMap.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flatcEncode(s)
	})
	testMap.testCall = test_serialize
	testMap.testVerifier = bytesEqual
	testMap.verbose = true
	t.Run(testMap.name, testMap.Verify)
	//Decoding
	testMap.MutateData(func(t *TestData) {
		b, ok := t.expectedResult.([]byte)
		if !ok {
			panic(fmt.Sprintf("Expected type []byte,got %T", t.expectedResult))
		}
		j, err := flatcDecode(&b)
		if err != nil {
			panic("Decoding failed")
		}
		t.expectedResult = string(j)
	})
	testMap.testCall = test_flatc_decode
	testMap.testVerifier = stringsEqual
	t.Run(testMap.name, testMap.Verify)
}

func TestAutoEncodingFlexeval(t *testing.T) {
	testMarshal := NewTestCase()
	testMarshal.name = "testMarshall"
	testMarshal.data = []TestData{
		{[]interface{}{1}, "Int(1) | EOF()"},
		{[]interface{}{[]interface{}{1, 2, 3}}, `StartVector() | Int(1) | Int(2) | Int(3) | EOF()`},
		{[]interface{}{"alpha"}, `String("alpha) | EOF()"`},
		{[]interface{}{[]interface{}{10, -20, 10.25}}, `StartVector() | Int(10) | Int(-20) | Int(10) | Float(10.25) | EndVector() | EOF()`},
		{[]interface{}{[]interface{}{[]interface{}{-1, -2, -3}, []interface{}{1, 2, 3}, "abc"}}, `StartVector() | StartVector() | Int(-1) | Int(-2) | Int(-3) | EndVector() | StartVector() | Int(1) | Int(2) | Int(3) | EndVector() | String("abc") | EndVector() | EOF()`},
		{[]interface{}{[]string{"This", "Is", "A", "Sentence"}}, `StartVector() | String("This") | String("Is") | String("A") | String("Sentence") | EndVector() | EOF()`},
		{[]interface{}{map[string]interface{}{"one": 1, "two": 2, "three": 3}}, `StartMap() | Key("one") | Int(1) | Key("two") | Int(2) | Key("three") | Int(3) | EndMap() | EOF()`},
		{[]interface{}{map[string]interface{}{"key1": "alpha", "key2": 2, "key3": -3, "key4": 10.25}}, `StartMap() | Key("key1") | String("alpha") | Key("key2") | Int(2) | Key("key3) | Int(-3) | Key("key4") | Float(10.25) | EndMap() | EOF()`},
		{[]interface{}{map[string]interface{}{"k1": []int{1, 2, 3}, "k2": map[string]interface{}{"K1": []int{3, 2, 1}, "K2": "XXX", "K3": math.MaxInt64}, "k3": "OK"}},
			fmt.Sprintf(`StartMap() | Key("k1") | StartVector() | Int(1) | Int(2) | Int(3) | EndVector() | Key("k2") | StartMap() | Key("K1") | StartVector() | Int(3) | Int(2) | Int(1) | EndVector() | Key("K2") | String("XXX") | Key("K3") | Int(%d) | | EndMap() | Key("k3") | String("OK") | EndMap() | EOF()`, math.MaxInt64)},
	}
	testMarshal.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flatcEncode(s)
	})
	testMarshal.testVerifier = bytesEqual
	testMarshal.testCall = func(t *testing.T, args ...interface{}) interface{} {
		assert.Assert(t, len(args) == 1, fmt.Sprintf("expected 1 argument, got %d", len(args)))
		buff, err := Marshal(args[0])
		require.NoError(t, err, "marshal has failed")
		return buff
	}
	t.Run(testMarshal.name, testMarshal.Verify)
}

func TestAutoEncodingFlatc(t *testing.T) {
	testMarshal := NewTestCase()
	testMarshal.name = "testMarshall"
	testMarshal.data = []TestData{
		{[]interface{}{1}, "1"},
		{[]interface{}{[]interface{}{1, 2, 3}}, "[1,2,3]"},
		{[]interface{}{"alpha"}, `"alpha"`},
		{[]interface{}{[]interface{}{10, -20, 10.25}}, "[10,-20,10.25]"},
		{[]interface{}{[]interface{}{[]interface{}{-1, -2, -3}, []interface{}{1, 2, 3}, "abc"}}, `[[-1,-2,-3],[1,2,3],"abc"]`},
		{[]interface{}{[]string{"This", "Is", "A", "Sentence"}}, `["This","Is","A","Sentence"]`},
		{[]interface{}{map[string]interface{}{"one": 1, "two": 2, "three": 3}}, `{one:1,two:2,three:3}`}, //TODO:sort keys alphanumerically
		{[]interface{}{map[string]interface{}{"key1": "alpha", "key2": 2, "key3": -3, "key4": 10.25}}, `{key1:"alpha",key2:2,key3:-3,key4:10.25}`},
		{[]interface{}{map[string]interface{}{"k1": []int{1, 2, 3}, "k2": map[string]interface{}{"K1": []int{3, 2, 1}, "K2": "XXX", "K3": math.MaxInt64}, "k3": "OK"}},
			fmt.Sprintf(`{k1:[1,2,3],k2:{K1:[3,2,1],K2:"XXX",K3:%d},k3:"OK"}`, math.MaxInt64)},
	}
	testMarshal.MutateData(func(t *TestData) {
		s, ok := t.expectedResult.(string)
		if !ok {
			panic("Expected string")
		}
		t.expectedResult = flatcEncode(s)
	})
	testMarshal.testVerifier = bytesEqual
	testMarshal.testCall = func(t *testing.T, args ...interface{}) interface{} {
		assert.Assert(t, len(args) == 1, fmt.Sprintf("expected 1 argument, got %d", len(args)))
		buff, err := Marshal(args[0])
		require.NoError(t, err, "marshal has failed")
		return buff
	}
	t.Run(testMarshal.name, testMarshal.Verify)
}

/*
func TestAB(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{
			name: "bool true",
			in:   `true`,
		},
		{
			name: "bool false",
			in:   `false`,
		},
		{
			name: "8-bit integer",
			in:   `13`,
		},
		{
			name: "16-bit integer",
			in:   `4242`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outA, err := encodeA(json.RawBytes(tt.in))
			require.NoError(t, err)

			outB, err := encodeB(json.RawBytes(tt.in))
			require.NoError(t, err)

			require.Equal(t, outA, outB)
		})
	}
}
*/
func encodeA(v json.RawMessage) ([]byte, error) {
	tempdir, err := ioutil.TempDir("", "flexbuffers-test.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempdir)

	if err := ioutil.WriteFile(filepath.Join(tempdir, "v.json"), v, 0666); err != nil {
		return nil, err
	}

	cmd := exec.Command("flatc", "--binary", "--flexbuffers", "--json", "v.json")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = tempdir
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return ioutil.ReadFile(filepath.Join(tempdir, "v.bin"))
}

func flatcDecode(buff *[]byte) (json.RawMessage, error) {

	tempdir, err := ioutil.TempDir("", "flexbuffers-test.")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempdir)

	if err := ioutil.WriteFile(filepath.Join(tempdir, "v.bin"), *buff, 0666); err != nil {
		return nil, err
	}

	cmd := exec.Command("flatc", "--flexbuffers", "--json", "v.bin")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.Dir = tempdir
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	b, err := ioutil.ReadFile(filepath.Join(tempdir, "v.json"))
	if err != nil {
		return nil, err
	}
	var m json.RawMessage
	if err := m.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return m, nil
}

func BenchmarkInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		B := NewBuilder()
		B.Int(1)
		/*		var buff []byte
				_, err := B.SerializeBuffer(&buff)
				if err != nil {
					b.Fatal(err)
				}*/
	}
}

func BenchmarkNewBuilder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		B := NewBuilder()
		_ = B
	}
}

/*
func encodeB(v json.RawMessage) ([]byte, error) {
	var vv interface{}
	if err := json.Unmarshal(v, &vv); err != nil {
		return nil, err
	}
	enc := &Encoder{}
	if err := enc.Write(vv); err != nil {
		return nil, err
	}
	return enc.Buffer.Bytes(), nil
}
*/
//TODO :foreginDecode - encode with go flex - decode with other language

// func NewBuilder() *Builder
//

// func Marshal(v interface{}) ([]byte, error)
//
// => Builder + reflect

// P1. It compiles
// P2. Marshal (autoBuilder)
// P3. Unit tests using Marshal (Go-Py consistency)
