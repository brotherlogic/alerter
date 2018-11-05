package main

import (
	"fmt"
	"time"

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

						if err == nil && len(latest.Versions) > 0 {
							if latest.Versions[0].Version != runningVersion && len(runningVersion) > 0 {
								s.lastMismatchTime[service.Identifier+job.Job.Name] = time.Now().Unix()
								s.alertCount++

								s.RaiseIssue(ctx, "Version Problem", fmt.Sprintf("%v is running an old version (%v vs %v)", job.Job.Name, runningVersion, latest.Versions[0].Version), false)

							} else {
								delete(s.lastMismatchTime, service.Identifier+job.Job.Name)
							}

						}

					}
				}
			}
		}
	}
}

func (s *Server) lookForSimulBuilds(ctx context.Context) {
	s.Log("Looking for concurrent builds")
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

func (s *Server) lookForHighCPU(ctx context.Context) {
	s.Log("Looking for high CPU usage")

	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})
	if err == nil {
		seen := make(map[string]bool)

		for _, service := range serv.Services.Services {
			if !seen[service.Name] {
				stats, err := s.goserver.GetStats(ctx, service.Name)

				if err == nil {
					for _, state := range stats.States {
						if state.Key == "cpu" {
							if state.Fraction > float64(80) {
								s.alertCount++
								s.highCPU[service.Name+service.Identifier] = time.Now().Unix()
								s.RaiseIssue(ctx, "High CPU", fmt.Sprintf("%v (%v) is reporting high cpu: %v", service.Name, service.Identifier, state.Fraction), false)
							} else {
								delete(s.highCPU, service.Name+service.Identifier)
							}
						}
					}
				}
			}
		}
	}
}

func (s *Server) lookForGoVersion(ctx context.Context) {
	s.Log("Looking for high CPU usage")

	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})
	if err == nil {
		for _, service := range serv.Services.Services {
			if service.Name == "gobuildslave" {
				stats, err := s.goserver.GetStats(ctx, service.Name)

				if err == nil {
					seen := false
					for _, state := range stats.States {
						if state.Key == "go_version" && state.Text != "go1.9" {
							s.alertCount++
							s.RaiseIssue(ctx, "Bad Version", fmt.Sprintf("%v (%v) is on the wrong go version", service.Identifier, state.Text), false)
						}
						if state.Key == "go_version" {
							seen = true
						}
					}
					if !seen {
						s.alertCount++
						s.RaiseIssue(ctx, "No Version", fmt.Sprintf("%v is not reporting a go version", service.Identifier), false)
					}
				}
			}
		}
	}
}
