package cc

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ClusterCockpit/cc-slurm-adapter/internal/config"
	slurm_common "github.com/ClusterCockpit/cc-slurm-adapter/internal/slurm/common"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (fn fixtureTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

type fixtureSlurm struct{ slurm_common.SlurmApi }

func (fixtureSlurm) GetClusterNames() []string                             { return []string{"example"} }
func (fixtureSlurm) QueryJobs(string, []int64) ([]slurm_common.Job, error) { return nil, nil }
func (fixtureSlurm) QueryJobsWithResources(string, []slurm_common.Job) error { return nil }

type faultBody struct {
	payload []byte
	failure error
	closed  bool
}

func (body *faultBody) Read(dst []byte) (int, error) {
	n := copy(dst, body.payload)
	body.payload = body.payload[n:]
	return n, body.failure
}
func (body *faultBody) Close() error { body.closed = true; return nil }

func TestCacheUpdateRejectsReadErrorAfterValidJSON(t *testing.T) {
	old := config.Config
	t.Cleanup(func() { config.Config = old })
	config.Config.CcRestUrl = "http://fixture.invalid"
	failure := errors.New("synthetic response read failure")
	body := &faultBody{payload: []byte(`{"jobs":[],"items":0,"page":1}`), failure: failure}
	api := CCApi{slurmApi: fixtureSlurm{}, httpClient: http.Client{Transport: fixtureTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "fixture.invalid" || req.Method != "GET" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})}}
	if err := api.CacheUpdate(); !errors.Is(err, failure) {
		t.Fatalf("read failure was accepted: %v", err)
	}
	if api.JobCacheValid || api.JobCache != nil {
		t.Fatal("incomplete response changed cache")
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

