// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-slurm-adapter.
// Use of this source code is governed by a MIT-style license.

package slurm_v24xx

import (
	"reflect"
	"testing"
)

func TestParseGRESRejectsMalformedIndexRanges(t *testing.T) {
	for _, input := range []string{
		"gpu:1(IDX:0-)", "gpu:1(IDX:0--1)", "gpu:1(IDX:2-1)",
		"gpu:2(IDX:0,1-)", "gpu:2(IDX:0,2-1)", "gpu:2(IDX:0,,1)",
		"gpu:1(IDX:,0)", "gpu:1(IDX:0,)", "gpu:1(IDX:-0)",
		"gpu:1(IDX:999999999999999999999999999999999999999)",
	} {
		if parsed, err := ParseGRES(input); err == nil || parsed != nil {
			t.Errorf("malformed index list %q: parsed=%+v err=%v", input, parsed, err)
		}
	}
}

func TestParseGRESPreservesValidResourceIndices(t *testing.T) {
	for _, tc := range []struct {
		input, variant, model, domain string
		count                         uint64
		indices                       []int
	}{
		{"gpu:h100:4(IDX:0-3)", "gpu", "h100", "IDX", 4, []int{0, 1, 2, 3}},
		{"gpu:2(IDX:1,3)", "gpu", "", "IDX", 2, []int{1, 3}},
		{"gpu:h100:4(S:0-1)", "gpu", "h100", "S", 4, []int{0, 1}},
		{"gpu:6(IDX:0-2,4,6-7)", "gpu", "", "IDX", 6, []int{0, 1, 2, 4, 6, 7}},
		{"gpu:2(IDX:1,1)", "gpu", "", "IDX", 2, []int{1, 1}},
		{"gpu:3(IDX:3,0-1)", "gpu", "", "IDX", 3, []int{3, 0, 1}},
		{"gpu:2(IDX:000-001)", "gpu", "", "IDX", 2, []int{0, 1}},
		{"gpu:0(IDX:0)", "gpu", "", "IDX", 0, []int{0}},
		{"gpu:18446744073709551615(IDX:0)", "gpu", "", "IDX", ^uint64(0), []int{0}},
	} {
		got, err := ParseGRES(tc.input)
		if err != nil || got == nil {
			t.Fatalf("valid GRES %q: parsed=%+v err=%v", tc.input, got, err)
		}
		if got.Variant != tc.variant || got.Id != tc.model || got.DomainType != tc.domain || got.Count != tc.count || !reflect.DeepEqual(got.DomainIndices, tc.indices) {
			t.Errorf("valid GRES %q: got=%+v want count=%d indices=%v", tc.input, got, tc.count, tc.indices)
		}
	}
	// Keep existing rejection of empty or unavailable domain representations.
	for _, input := range []string{"", "gpu:0(IDX:)", "gpu:0(IDX:N/A)", "gpu:1(IDX:text)", "gpu:18446744073709551616(IDX:0)"} {
		if _, err := ParseGRES(input); err == nil {
			t.Errorf("previously rejected representation accepted: %q", input)
		}
	}
}
