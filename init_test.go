package main

import (
	"strings"
	"testing"

	"github.com/go-test/deep"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type checker struct {
	info *CheckerInfo
	code func([]interface{}, []string) (bool, string)
}

func (c checker) Info() *CheckerInfo { return c.info }
func (c checker) Check(params []interface{}, names []string) (result bool, error string) {
	return c.code(params, names)
}

func init() {
	DeepEquals = checker{
		info: &CheckerInfo{Name: "DeepEqualsPP", Params: []string{"obtained", "expected"}},
		code: func(params []interface{}, names []string) (result bool, error string) {
			if diff := deep.Equal(params[0], params[1]); len(diff) > 0 {
				return false, "... ...\n  " + strings.Join(diff, "\n  ")
			}
			return true, ""
		},
	}
}
