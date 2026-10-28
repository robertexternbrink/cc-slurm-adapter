package cc

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ClusterCockpit/cc-lib/v2/schema"
	"github.com/ClusterCockpit/cc-slurm-adapter/internal/config"
	slurm_common "github.com/ClusterCockpit/cc-slurm-adapter/internal/slurm/common"
)

type resyncSlurm struct {
	fixtureSlurm
	failure error
}

func (api resyncSlurm) QueryJobs(_ string, ids []int64) ([]slurm_common.Job, error) {
	if api.failure != nil {
		return nil, api.failure
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return []slurm_common.Job{resyncRunningJob{}}, nil
}
func (resyncSlurm) QueryJobsWithResources(string, []slurm_common.Job) error { return nil }

type resyncRunningJob struct{ fixtureJob }

func (resyncRunningJob) GetState() string { return "RUNNING" }
func (resyncRunningJob) IsFinished() bool { return false }
func (resyncRunningJob) GetResources() ([]*schema.Resource, error) {
	return []*schema.Resource{{Hostname: "node01", HWThreads: []int{0}}}, nil
}
func (resyncRunningJob) GetName() string             { return "fixture" }
func (resyncRunningJob) GetSlurmInfo() string        { return "" }
func (resyncRunningJob) GetSubmitTime() time.Time    { return time.Unix(1, 0) }
func (resyncRunningJob) GetJobScript() string        { return "" }
func (resyncRunningJob) GetPartition() string        { return "fixture" }
func (resyncRunningJob) GetAccount() string          { return "fixture" }
func (resyncRunningJob) GetArrayJobId() int64        { return 0 }
func (resyncRunningJob) GetNumNodes() int32          { return 1 }
func (resyncRunningJob) GetNumHWThreads() int32      { return 1 }
func (resyncRunningJob) GetNumAccelerators() int32   { return 0 }
func (resyncRunningJob) GetNodeShared() string       { return "none" }
func (resyncRunningJob) GetTimeLimit() time.Duration { return time.Minute }
func (resyncRunningJob) GetUser() string             { return "fixture" }
func (resyncRunningJob) GetStartTime() time.Time     { return time.Unix(2, 0) }

func TestCacheUpdateRestartsKnownJobMissingFromBackend(t *testing.T) {
	old := config.Config
	t.Cleanup(func() { config.Config = old })
	config.Config = config.ProgramConfig{CcRestUrl: "http://fixture.invalid", CcRestSubmitJobs: true}
	for _, status := range []int{201, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			starts := 0
			api := CCApi{slurmApi: resyncSlurm{}, JobCache: map[string]map[int64]*CacheJobState{"example": {123: {Running: true}}}, httpClient: http.Client{Transport: fixtureTransport(func(req *http.Request) (*http.Response, error) {
				switch req.Method + " " + req.URL.Path {
				case "GET /api/jobs/":
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jobs":[],"items":0,"page":1}`))}, nil
				case "POST /api/jobs/start_job/":
					starts++
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("synthetic response"))}, nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
					return nil, nil
				}
			})}}
			err := api.CacheUpdate()
			if starts != 1 {
				t.Fatalf("missing backend job was not restarted: starts=%d", starts)
			}
			if status == 201 {
				if err != nil || !api.JobCacheValid || !api.JobCache["example"][123].Running {
					t.Fatalf("successful resync failed: %v", err)
				}
			} else {
				if err == nil || api.JobCacheValid {
					t.Fatalf("failed restart accepted as valid cache: %v", err)
				}
				if state := api.JobCache["example"][123]; state == nil || !state.Running {
					t.Fatal("failed restart destroyed retry state")
				}
				status = 201
				if err := api.CacheUpdate(); err != nil || starts != 2 || !api.JobCacheValid || !api.JobCache["example"][123].Running {
					t.Fatalf("retry failed: starts=%d err=%v", starts, err)
				}
			}
		})
	}
}

func TestCacheUpdateKeepsRetryStateOnSlurmFailure(t *testing.T) {
	old := config.Config
	t.Cleanup(func() { config.Config = old })
	config.Config = config.ProgramConfig{CcRestUrl: "http://fixture.invalid", CcRestSubmitJobs: true}
	failure := errors.New("synthetic accounting failure")
	for _, backendHasJob := range []bool{false, true} {
		api := CCApi{slurmApi: resyncSlurm{failure: failure}, JobCacheValid: true, JobCache: map[string]map[int64]*CacheJobState{"example": {123: {Running: !backendHasJob}}}, httpClient: http.Client{Transport: fixtureTransport(func(req *http.Request) (*http.Response, error) {
			if req.Method != "GET" {
				t.Fatalf("failed accounting query caused submission: %s", req.Method)
			}
			body := `{"jobs":[],"items":0,"page":1}`
			if backendHasJob {
				body = `{"jobs":[{"cluster":"example","jobId":123,"state":"running"}],"items":1,"page":1}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}}
		if err := api.CacheUpdate(); !errors.Is(err, failure) || api.JobCacheValid {
			t.Fatalf("accounting failure accepted: %v", err)
		}
		if state := api.JobCache["example"][123]; state == nil || state.Running != !backendHasJob {
			t.Fatal("query failure changed cached retry state")
		}
	}
}
