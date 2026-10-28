package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ClusterCockpit/cc-lib/v2/schema"
	"github.com/ClusterCockpit/cc-slurm-adapter/internal/cc"
	"github.com/ClusterCockpit/cc-slurm-adapter/internal/config"
	slurm_common "github.com/ClusterCockpit/cc-slurm-adapter/internal/slurm/common"
)

type checkpointSlurm struct {
	slurm_common.SlurmApi
	fail   bool
	starts []time.Time
}

func (*checkpointSlurm) GetClusterNames() []string { return []string{"first", "second"} }
func (api *checkpointSlurm) QueryJobsTimeRange(cluster string, begin, end time.Time) ([]slurm_common.Job, error) {
	api.starts = append(api.starts, begin)
	if cluster == "second" && api.fail {
		return nil, errors.New("synthetic second-cluster failure")
	}
	return []slurm_common.Job{checkpointJob{cluster: cluster}}, nil
}
func (*checkpointSlurm) QueryJobsWithResources(string, []slurm_common.Job) error { return nil }

type checkpointJob struct {
	slurm_common.Job
	cluster string
}

func (job checkpointJob) GetCluster() string                    { return job.cluster }
func (checkpointJob) GetJobId() int64                           { return 123 }
func (checkpointJob) GetState() string                          { return "RUNNING" }
func (checkpointJob) GetResources() ([]*schema.Resource, error) { return nil, nil }

func TestSacctCheckpointCommitsAfterEveryClusterSucceeds(t *testing.T) {
	oldConfig, oldSlurm, oldCC := config.Config, slurmApi, ccApi
	t.Cleanup(func() { config.Config, slurmApi, ccApi = oldConfig, oldSlurm, oldCC })
	config.Config = config.ProgramConfig{LastRunPath: filepath.Join(t.TempDir(), "last_run"), SlurmQueryMaxSpan: 7 * 86400, CcRestSubmitJobs: true}
	checkpoint := time.Now().Add(-time.Hour).Truncate(time.Second)
	lastRunSet(checkpoint)
	fake := &checkpointSlurm{fail: true}
	slurmApi = fake
	var err error
	ccApi, err = cc.NewCCApi(fake)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ccApi.Close)
	ccApi.JobCache = map[string]map[int64]*cc.CacheJobState{"first": {}, "second": {}}
	// All fake jobs have no resources and are deliberately filtered before API submission.
	if err := processSlurmSacctPoll(); err == nil {
		t.Fatal("second-cluster failure accepted")
	}
	info, err := os.Stat(config.Config.LastRunPath)
	if err != nil || !info.ModTime().Equal(checkpoint) {
		t.Fatalf("failed multi-cluster poll advanced checkpoint: %v, %v", info, err)
	}
	fake.fail = false
	if err := processSlurmSacctPoll(); err != nil {
		t.Fatal(err)
	}
	if !lastRunGet().After(checkpoint) {
		t.Fatal("successful poll did not advance checkpoint")
	}
	for _, begin := range fake.starts {
		if !begin.Equal(checkpoint.Add(-time.Second)) {
			t.Fatalf("retry lost time range: begin=%s checkpoint=%s", begin, checkpoint)
		}
	}
}
