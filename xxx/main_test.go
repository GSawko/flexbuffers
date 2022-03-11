package main

import (
	"fmt"
	"strconv"
	"testing"
)

func TestAtof(t *testing.T) {
	str1 := `"340282346638528859811704183484516925440.000000"`
	_float := func(str string) float64 {
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return f
		}
		panic(fmt.Sprintf("could not parse '%s' to float64", str))
	}
	f := _float(str1)
	t.Logf("%f", f)
	str2 := fmt.Sprintf("%f", f)
	if str1 != str2 {
		t.Fatalf("%s\n %s\n %f\n", str1, str2, f)
	}

}
