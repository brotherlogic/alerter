package main

import (
	"fmt"
	"testing"
	"time"

	pbg "github.com/brotherlogic/goserver/proto"
	"github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"

	pbbs "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbgbs "github.com/brotherlogic/gobuildslave/proto"
)

type testDiscovery struct {
	failget    bool
	failremote bool
}

func (t *testDiscovery) ListAllServices(ctx context.Context, req *pbd.ListRequest) (*pbd.ListResponse, error) {
	return &pbd.ListResponse{Services: &pbd.ServiceList{Services: []*pbd.RegistryEntry{&pbd.RegistryEntry{Name: "gobuildslave", Ip: "1234", Port: int32(123)}}}}, nil
}

func (t *testDiscovery) getFriends(ctx context.Context) (string, error) {
	if t.failget && !t.failremote {
		return "", fmt.Errorf("Built to fail")
	}
	return "yeps", nil
}

func (t *testDiscovery) getRemoteFriends(ctx context.Context, addr string) (string, error) {
	if t.failremote && !t.failget {
		return "", fmt.Errorf("Built to fail")
	}
	return "yep", nil
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
	reportsNormal    bool
	goversion        bool
	concurrentBuilds int64
}

func (t *testGoserver) GetStats(ctx context.Context, server string, port int32) (*pbg.ServerState, error) {
	if t.reportsNormal {
		if t.goversion {
			return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "go_version", Text: "go1.11.6"}, &pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
		}
		return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "go_version", Text: "badversion"}, &pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
	}
	return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(50)}}}, nil
}

func (t *testGoserver) GetStatsSingle(ctx context.Context, server string) (*pbg.ServerState, error) {
	if t.reportsNormal {
		if t.goversion {
			return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "go_version", Text: "go1.11.6"}, &pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
		}
		return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "go_version", Text: "badversion"}, &pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
	}
	return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "concurrent_builds", Value: t.concurrentBuilds}, &pbg.State{Key: "cpu", Fraction: float64(50)}}}, nil
}

func InitTestServer() *Server {
	s := Init()
	s.discover = &testDiscovery{}
	s.buildServer = &testBuildserver{}
	s.gobuildSlave = &testGobuildslave{}
	s.SkipLog = true
	s.SkipIssue = true
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

	if s.alertCount != 0 {
		t.Errorf("Error in alerting: %v", s.alertCount)
	}
}

func TestBuildAlertFires(t *testing.T) {
	s := InitTestServer()
	s.goserver = &testGoserver{concurrentBuilds: 5}
	s.lookForSimulBuilds(context.Background())

	if s.alertCount == 0 {
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

func TestDisc(t *testing.T) {
	s := InitTestServer()
	err := s.checkFriends(context.Background())

	if err != nil {
		t.Errorf("Bad check: %v", err)
	}
}

func TestDiscFailLocal(t *testing.T) {
	s := InitTestServer()
	s.discover = &testDiscovery{failget: true}
	err := s.checkFriends(context.Background())

	if err == nil {
		t.Errorf("Bad check: %v", err)
	}
}

func TestDiscFailRemote(t *testing.T) {
	s := InitTestServer()
	s.discover = &testDiscovery{failremote: true}
	err := s.checkFriends(context.Background())

	if err == nil {
		t.Errorf("Bad check: %v", err)
	}
}
