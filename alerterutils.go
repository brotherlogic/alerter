package main

import (
	"fmt"

	pbbs "github.com/brotherlogic/buildserver/proto"
	"golang.org/x/net/context"

	pbgs "github.com/brotherlogic/gobuildslave/proto"

	pbd "github.com/brotherlogic/discovery/proto"
)

func (s *Server) runVersionCheck(ctx context.Context) {
	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})
	if err == nil {
		for _, service := range serv.Services.Services {
			if service.Name == "gobuildslave" {
				jobs, err := s.gobuildSlave.ListJobs(ctx, service, &pbgs.ListRequest{})
				if err == nil {
					for _, job := range jobs.Jobs {
						runningVersion := job.RunningVersion
						latest, err := s.buildServer.GetVersions(ctx, &pbbs.VersionRequest{Job: job.Job, JustLatest: true})
						if err == nil && latest != nil && len(latest.Versions) > 0 {
							s.Log(fmt.Sprintf("Checking these versions: %v", latest.Versions))
						}

						if err == nil && len(latest.Versions) > 0 && latest.Versions[0].Version != runningVersion {
							s.alertCount++
							s.RaiseIssue(ctx, "Version Problem", fmt.Sprintf("%v is running an old version (%v vs %v)", job.Job.Name, runningVersion, latest.Versions[0].Version), false)
						}
					}
				}
			}
		}
	}
}

func (s *Server) lookForSimulBuilds(ctx context.Context) {
	stats, err := s.goserver.GetStats(ctx, "buildserver")
	if err == nil {
		for _, state := range stats.States {
			if state.Key == "concurrent_builds" && state.Value > int64(1) {
				s.alertCount++
				s.RaiseIssue(ctx, "ConcurrentBuilds", fmt.Sprintf("Buildserver is reporting concurrent builds: %v", state.Value), false)
			}
		}
	}
}
