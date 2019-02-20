package main

import (
	"log"
	"testing"
	"time"

	pbg "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"

	pbbs "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbgbs "github.com/brotherlogic/gobuildslave/proto"
)

type testDiscovery struct{}

func (t *testDiscovery) ListAllServices(ctx context.Context, req *pbd.ListRequest) (*pbd.ListResponse, error) {
	return &pbd.ListResponse{Services: &pbd.ServiceList{Services: []*pbd.RegistryEntry{&pbd.RegistryEntry{Name: "gobuildslave", Ip: "1234", Port: int32(123)}}}}, nil
}

type testBuildserver struct {
	match bool
}

func (t *testBuildserver) GetVersions(ctx context.Context, req *pbbs.VersionRequest) (*pbbs.VersionResponse, error) {
	if !t.match {
		return &pbbs.VersionResponse{Versions: []*pbbs.Version{&pbbs.Version{Version: "testing", Job: &pbgbs.Job{Name: "madeup"}}}}, nil
	}
	return &pbbs.VersionResponse{Versions: []*pbbs.Version{&pbbs.Version{Version: "not_testing", Job: &pbgbs.Job{Name: "madeup"}}}}, nil
}

type testGobuildslave struct {
	job bool
}

func (t *testGobuildslave) ListJobs(ctx context.Context, server *pbd.RegistryEntry, req *pbgbs.ListRequest) (*pbgbs.ListResponse, error) {
	if !t.job {
		return &pbgbs.ListResponse{Jobs: []*pbgbs.JobAssignment{&pbgbs.JobAssignment{RunningVersion: "not_testing", Job: &pbgbs.Job{Name: "madeup"}}}}, nil
	}
	return &pbgbs.ListResponse{Jobs: []*pbgbs.JobAssignment{&pbgbs.JobAssignment{RunningVersion: "not_testing", Job: &pbgbs.Job{Name: "other"}}}}, nil
}

type testGoserver struct {
	reportsNormal bool
	goversion     bool
}

func (t *testGoserver) GetStats(ctx context.Context, server string) (*pbg.ServerState, error) {
	if t.reportsNormal {
		if t.goversion {
			return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "go_version", Text: "go1.9"}, &pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
		}
		return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "go_version", Text: "badversion"}, &pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
	}
	return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(50)}}}, nil
}

func InitTestServer() *Server {
	s := Init()
	s.discover = &testDiscovery{}
	s.buildServer = &testBuildserver{}
	s.gobuildSlave = &testGobuildslave{}
	s.SkipLog = true
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".test")
	s.goserver = &testGoserver{}
	return s
}

func TestAlert(t *testing.T) {
	s := InitTestServer()
	s.runVersionCheck(context.Background(), time.Hour)

	if s.alertCount != 0 {
		t.Errorf("Error in alerting")
	}
}

func TestAlertJobDiff(t *testing.T) {
	s := InitTestServer()
	s.gobuildSlave = &testGobuildslave{job: true}
	s.runVersionCheck(context.Background(), time.Hour)

	if s.alertCount != 0 {
		t.Errorf("Error in alerting")
	}
}

func TestAlertWithTime(t *testing.T) {
	s := InitTestServer()
	s.runVersionCheck(context.Background(), time.Hour)
	time.Sleep(time.Second)
	s.runVersionCheck(context.Background(), time.Millisecond)

	if s.alertCount != 1 {
		t.Errorf("Error in alerting")
	}
}

func TestAlertClear(t *testing.T) {
	s := InitTestServer()
	s.runVersionCheck(context.Background(), time.Hour)
	s.buildServer = &testBuildserver{match: true}
	s.runVersionCheck(context.Background(), time.Hour)

	if s.alertCount != 0 {
		t.Errorf("Error in alerting")
	}
}

func TestBuildAlert(t *testing.T) {
	s := InitTestServer()
	s.lookForSimulBuilds(context.Background())

	log.Printf("COUNT: %v", s.alertCount)
	if s.alertCount != 0 {
		t.Errorf("Error in alerting: %v", s.alertCount)
	}
}

func TestCPUAlert(t *testing.T) {
	s := InitTestServer()
	s.lookForHighCPU(context.Background(), time.Hour)
	if s.alertCount != 0 {
		t.Errorf("Error in alerting")
	}
}

func TestCPUAlertWithVersion(t *testing.T) {
	s := InitTestServer()
	s.goserver = &testGoserver{reportsNormal: true}
	s.lookForHighCPU(context.Background(), time.Hour)
	time.Sleep(time.Second)
	s.lookForHighCPU(context.Background(), time.Millisecond)
	if s.alertCount != 1 {
		t.Errorf("Error in alerting: %v", s.alertCount)
	}
}

func TestGoVersionAlert(t *testing.T) {
	s := InitTestServer()
	s.lookForGoVersion(context.Background())
	if s.alertCount != 1 {
		t.Errorf("Error in alerting: %v", s.alertCount)
	}
}

func TestGoVersionAlertMissing(t *testing.T) {
	s := InitTestServer()
	s.goserver = &testGoserver{reportsNormal: true}
	s.lookForGoVersion(context.Background())
	if s.alertCount != 1 {
		t.Errorf("Error in alerting: %v", s.alertCount)
	}
}

func TestGoVersionNoAlert(t *testing.T) {
	s := InitTestServer()
	s.goserver = &testGoserver{reportsNormal: true, goversion: true}
	s.lookForGoVersion(context.Background())
	if s.alertCount != 0 {
		t.Errorf("Error in alerting: %v", s.alertCount)
	}
}
