// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-slurm-adapter.
// Use of this source code is governed by a MIT-style license.

package slurm_v24xx

import (
	"encoding/json"
	"testing"
)

func TestSlurmStringDecodesStringsAndArrays(t *testing.T) {
	for _, fixture := range []struct{ raw, want string }{
		{`"RUNNING"`, "RUNNING"},
		{`["RUNNING"]`, "RUNNING"},
		{`"ALLOCATED"`, "ALLOCATED"},
		{`"quoted\"and\\escaped"`, "quoted\"and\\escaped"},
		{`"ü🙂"`, "ü🙂"},
		{`[]`, ""},
		{`null`, ""},
		{`["first","second"]`, "first"},
	} {
		var got SlurmString
		if err := json.Unmarshal([]byte(fixture.raw), &got); err != nil {
			t.Fatal(err)
		}
		if string(got) != fixture.want {
			t.Errorf("%s decoded as %q, want %q", fixture.raw, got, fixture.want)
		}
	}
}

func TestSlurmStringRejectsInvalidTypes(t *testing.T) {
	for _, raw := range []string{`42`, `true`, `{}`, `[42]`} {
		var got SlurmString
		if err := json.Unmarshal([]byte(raw), &got); err == nil {
			t.Errorf("invalid string representation %s accepted", raw)
		}
	}
}
