package main

import (
	"testing"

	"github.com/powerman/gocheckext"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { gocheckext.CountingTestingT(t) }

func init() {
	DeepEquals = gocheckext.DeepEqualsPP
}
