package main

import (
	"flag"
	"io/ioutil"
	"log"
	"strconv"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"github.com/brotherlogic/keystore/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pbbs "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbgbs "github.com/brotherlogic/gobuildslave/proto"
	pbg "github.com/brotherlogic/goserver/proto"
)

// Goserver interface to a server
type Goserver interface {
	GetStats(ctx context.Context, ip string, port int32) (*pbg.ServerState, error)
	GetStatsSingle(ctx context.Context, host string) (*pbg.ServerState, error)
}

type prodGoserver struct {
	dial func(server string) (*grpc.ClientConn, error)
}

func (p *prodGoserver) GetStatsSingle(ctx context.Context, server string) (*pbg.ServerState, error) {
	conn, err := p.dial(server)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := pbg.NewGoserverServiceClient(conn)
	return client.State(ctx, &pbg.Empty{})
}

func (p *prodGoserver) GetStats(ctx context.Context, ip string, port int32) (*pbg.ServerState, error) {
	conn, err := grpc.Dial(ip+":"+strconv.Itoa(int(port)), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := pbg.NewGoserverServiceClient(conn)
	return client.State(ctx, &pbg.Empty{})
}

// Discovery interface to discover
type Discovery interface {
	ListAllServices(ctx context.Context, req *pbd.ListRequest) (*pbd.ListResponse, error)
}

type prodDiscovery struct{}

func (p *prodDiscovery) ListAllServices(ctx context.Context, req *pbd.ListRequest) (*pbd.ListResponse, error) {
	conn, err := grpc.Dial(utils.Discover, grpc.WithInsecure())
	defer conn.Close()

	if err != nil {
		return nil, err
	}

	client := pbd.NewDiscoveryServiceClient(conn)
	return client.ListAllServices(ctx, req)
}

// BuildServer interface to buildserver
type BuildServer interface {
	GetVersions(ctx context.Context, req *pbbs.VersionRequest) (*pbbs.VersionResponse, error)
}

type prodBuildserver struct {
	dial func(server string) (*grpc.ClientConn, error)
}

func (p *prodBuildserver) GetVersions(ctx context.Context, req *pbbs.VersionRequest) (*pbbs.VersionResponse, error) {
	conn, err := p.dial("buildserver")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pbbs.NewBuildServiceClient(conn)
	return client.GetVersions(ctx, req)
}

//GobuildSlave interface to gbs
type GobuildSlave interface {
	ListJobs(ctx context.Context, server *pbd.RegistryEntry, req *pbgbs.ListRequest) (*pbgbs.ListResponse, error)
}

type prodGobuildSlave struct{}

func (p *prodGobuildSlave) ListJobs(ctx context.Context, server *pbd.RegistryEntry, req *pbgbs.ListRequest) (*pbgbs.ListResponse, error) {
	conn, err := grpc.Dial(server.Ip+":"+strconv.Itoa(int(server.Port)), grpc.WithInsecure())
	defer conn.Close()
	if err != nil {
		return nil, err
	}

	client := pbgbs.NewBuildSlaveClient(conn)
	return client.ListJobs(ctx, req)
}

//Server main server type
type Server struct {
	*goserver.GoServer
	buildServer      BuildServer
	gobuildSlave     GobuildSlave
	discover         Discovery
	alertCount       int
	goserver         Goserver
	lastMismatchTime map[string]time.Time
	highCPU          map[string]time.Time
}

// Init builds the server
func Init() *Server {
	s := &Server{
		&goserver.GoServer{},
		&prodBuildserver{},
		&prodGobuildSlave{},
		&prodDiscovery{},
		0,
		&prodGoserver{},
		make(map[string]time.Time),
		make(map[string]time.Time),
	}
	s.goserver = &prodGoserver{dial: s.DialMaster}
	s.buildServer = &prodBuildserver{dial: s.DialMaster}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {

}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

// Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{
		&pbg.State{Key: "blah", Value: int64(100)},
	}
}

func (s *Server) runVersionCheckLoop(ctx context.Context) error {
	s.runVersionCheck(ctx, time.Minute*20)
	return nil
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.GoServer.KSclient = *keystoreclient.GetClient(server.DialMaster)
	server.PrepServer()
	server.Register = server
	//server.SendTrace = true

	server.RegisterServer("alerter", false)
	server.RegisterRepeatingTask(server.runVersionCheckLoop, "run_version_check", time.Minute)
	server.RegisterRepeatingTask(server.lookForSimulBuilds, "look_for_simul_builds", time.Minute)
	server.RegisterRepeatingTask(server.lookForGoVersion, "look_for_go_version", time.Hour)
	server.Log("Starting Alerter!")
	server.Serve()
}
