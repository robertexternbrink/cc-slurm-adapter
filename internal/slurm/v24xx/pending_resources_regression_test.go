// Copyright (C) NHR@FAU, University Erlangen-Nuremberg.
// All rights reserved. This file is part of cc-slurm-adapter.
// Use of this source code is governed by a MIT-style license.

package slurm_v24xx

import (
	"reflect"
	"testing"

	"github.com/ClusterCockpit/cc-slurm-adapter/internal/config"
)

func TestGetResourcesScOnlyPending(t *testing.T) {
	id, cluster := int64(31), "fixture-cluster"
	for _, pending := range []*ScontrolJobResources{nil, {Nodes: nil}} {
		job := &Job{sc: &ScontrolJob{JobId: &id, Cluster: &cluster, JobResources: pending}}
		resources, err := job.GetResources()
		if err != nil || len(resources) != 0 {
			t.Errorf("pending job resources=%v err=%v", resources, err)
		}
	}
}

func TestGetResourcesScOnlyGpuMappingErrors(t *testing.T) {
	original := config.Config
	defer func() { config.Config = original }()
	id, cluster, host, comment := int64(31), "fixture-cluster", "fixture-node", ""
	zero, status := 0, SlurmString("ALLOCATED")
	for _, tc := range []struct {
		filter string
		want   []string
	}{
		{"^fixture-node$", []string{"0000:65:00.0"}},
		{"^other-node$", nil},
	} {
		config.Config = config.ProgramConfig{HostSettings: map[string]config.HostConfig{
			tc.filter: {GpuPciAddrs: []string{"0000:65:00.0"}},
		}}
		job := &Job{sc: &ScontrolJob{
			JobId: &id, Cluster: &cluster, Comment: &comment,
			GresDetail: []string{"gpu:2(IDX:0,2)"},
			JobResources: &ScontrolJobResources{Nodes: &ScontrolJobResourcesNodes{Allocation: []ScontrolJobResourcesNodesAllocation{{
				Hostname: &host, Index: &zero,
				Sockets: []ScontrolJobResourcesNodesAllocationSocket{{Index: &zero, Cores: []ScontrolJobResourcesNodesAllocationSocketCore{{Index: &zero, Status: &status}}}},
			}}}},
		}}
		got, err := job.GetResources()
		if err != nil || len(got) != 1 {
			t.Fatalf("resources=%v err=%v", got, err)
		}
		if !reflect.DeepEqual(got[0].Accelerators, tc.want) || !reflect.DeepEqual(got[0].HWThreads, []int{0}) {
			t.Fatalf("unexpected resource mapping: %+v", got[0])
		}
	}
}
