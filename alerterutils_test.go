package main

import (
	"testing"

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
	return &pbgbs.ListResponse{Jobs: []*pbgbs.JobAssignment{&pbgbs.JobAssignment{RunningVersion: "not_testing"}}}, nil
}

func InitTestServer() *Server {
	s := Init()
	s.discover = &testDiscovery{}
	s.buildServer = &testBuildserver{}
	s.gobuildSlave = &testGobuildslave{}
	s.SkipLog = true
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".test")
	return s
}

func TestAlert(t *testing.T) {
	s := InitTestServer()
	s.runVersionCheck(context.Background())

	if s.alertCount != 1 {
		t.Errorf("Error in alerting")
	}
}
