package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"

	pbbs "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbgs "github.com/brotherlogic/gobuildslave/proto"
)

func (s *Server) checkFriends(ctx context.Context) error {
	friends, err := s.discover.getFriends(ctx)
	if err != nil {
		return err
	}

	for _, friend := range strings.Split(friends, " ") {
		rfriends, err := s.discover.getRemoteFriends(ctx, strings.Replace(strings.Replace(friend, "[", "", -1), "]", "", -1))
		if err != nil {
			return err
		}
		if len(rfriends) != len(friends) {
			s.RaiseIssue(ctx, "Friend mismatch", fmt.Sprintf("%v != %v", friends, rfriends), false)
		}
	}

	return nil
}

func (s *Server) runVersionCheck(ctx context.Context, delay time.Duration) error {
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
								return nil
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

	return err
}

func (s *Server) lookForSimulBuilds(ctx context.Context) error {
	s.Log("Looking for concurrent builds")
	stats, err := s.goserver.GetStatsSingle(ctx, "buildserver")
	if err == nil {
		for _, state := range stats.States {
			if state.Key == "concurrent_builds" && state.Value > int64(2) {
				s.alertCount++
				s.RaiseIssue(ctx, "ConcurrentBuilds", fmt.Sprintf("Buildserver is reporting concurrent builds: %v", state.Value), false)
			}
		}
	}
	return nil
}

func (s *Server) lookForGoVersion(ctx context.Context) error {
	s.Log("Looking for high CPU usage")

	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})
	if err == nil {
		for _, service := range serv.Services.Services {
			if service.Name == "gobuildslave" {
				stats, err := s.goserver.GetStats(ctx, service.Ip, service.Port)

				if err == nil {
					seen := false
					for _, state := range stats.States {
						if state.Key == "go_version" && state.Text != "go1.11.6" {
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

	return nil
}
