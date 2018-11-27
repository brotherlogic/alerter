package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	pbbs "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbgs "github.com/brotherlogic/gobuildslave/proto"
	pbt "github.com/brotherlogic/tracer/proto"
)

func (s *Server) runVersionCheck(ctx context.Context, delay time.Duration) {
	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})
	versionMap := make(map[string]string)
	if err == nil {
		versions, err := s.buildServer.GetVersions(ctx, &pbbs.VersionRequest{JustLatest: true})
		if err == nil {
			for _, v := range versions.Versions {
				versionMap[v.Job.Name] = v.Version
			}
			for _, service := range serv.Services.Services {
				if service.Name == "gobuildslave" {
					jobs, err := s.gobuildSlave.ListJobs(ctx, service, &pbgs.ListRequest{})
					if err == nil {
						for _, job := range jobs.Jobs {
							runningVersion := job.RunningVersion
							compiledVersion, ok := versionMap[job.Job.Name]
							if !ok {
								s.RaiseIssue(ctx, "Version Problem", fmt.Sprintf("%v has no version built", job.Job.Name), false)
								return
							}
							if compiledVersion != runningVersion && len(runningVersion) > 0 {
								if _, ok := s.lastMismatchTime[service.Identifier+job.Job.Name]; !ok {
									s.lastMismatchTime[service.Identifier+job.Job.Name] = time.Now()
								}

								if time.Now().Sub(s.lastMismatchTime[service.Identifier+job.Job.Name]) > delay {
									s.alertCount++
									s.RaiseIssue(ctx, "Version Problem", fmt.Sprintf("%v is running an old version (%v vs %v)", job.Job.Name, runningVersion, compiledVersion), false)
								}

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

func (s *Server) lookForHighCPU(ctx context.Context, delay time.Duration) {
	ctx = s.LogTrace(ctx, "lookForHighCPU", time.Now(), pbt.Milestone_START_FUNCTION)

	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})

	s.LogTrace(ctx, "ListedServices", time.Now(), pbt.Milestone_MARKER)
	if err == nil {
		seen := make(map[string]bool)

		for _, service := range serv.Services.Services {
			if !seen[service.Name] {
				stats, err := s.goserver.GetStats(ctx, service.Name)

				if err == nil {
					for _, state := range stats.States {
						if state.Key == "cpu" {
							if state.Fraction > float64(80) {
								if _, ok := s.highCPU[service.Name+service.Identifier]; !ok {
									s.highCPU[service.Name+service.Identifier] = time.Now()
								}

								if time.Now().Sub(s.highCPU[service.Name+service.Identifier]) > delay {
									s.alertCount++
									s.RaiseIssue(ctx, "High CPU", fmt.Sprintf("%v (%v) is reporting high cpu: %v", service.Name, service.Identifier, state.Fraction), false)
								}
							} else {
								delete(s.highCPU, service.Name+service.Identifier)
							}
						}
					}
				}
			}
		}
	}
	s.LogTrace(ctx, "lookForHighCPU", time.Now(), pbt.Milestone_END_FUNCTION)
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
