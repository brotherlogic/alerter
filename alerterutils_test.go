package main

import (
	"log"
	"testing"

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

type testBuildserver struct{}

func (t *testBuildserver) GetVersions(ctx context.Context, req *pbbs.VersionRequest) (*pbbs.VersionResponse, error) {
	return &pbbs.VersionResponse{Versions: []*pbbs.Version{&pbbs.Version{Version: "testing"}}}, nil
}

type testGobuildslave struct{}

func (t *testGobuildslave) ListJobs(ctx context.Context, server *pbd.RegistryEntry, req *pbgbs.ListRequest) (*pbgbs.ListResponse, error) {
	return &pbgbs.ListResponse{Jobs: []*pbgbs.JobAssignment{&pbgbs.JobAssignment{RunningVersion: "not_testing", Job: &pbgbs.Job{Name: "madeup"}}}}, nil
}

type testGoserver struct{}

func (t *testGoserver) GetStats(ctx context.Context, server string) (*pbg.ServerState, error) {
	return &pbg.ServerState{States: []*pbg.State{&pbg.State{Key: "concurrent_builds", Value: int64(2)}, &pbg.State{Key: "cpu", Fraction: float64(200)}}}, nil
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
	s.runVersionCheck(context.Background())

	if s.alertCount != 1 {
		t.Errorf("Error in alerting")
	}
}

func TestBuildAlert(t *testing.T) {
	s := InitTestServer()
	s.lookForSimulBuilds(context.Background())

	log.Printf("COUNT: %v", s.alertCount)
	if s.alertCount != 1 {
		t.Errorf("Error in alerting")
	}
}

func TestCPUAlert(t *testing.T) {
	s := InitTestServer()
	s.lookForHighCPU(context.Background())
	if s.alertCount != 1 {
		t.Errorf("Error in alerting")
	}
}
