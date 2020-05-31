package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"golang.org/x/net/context"

	pbbs "github.com/brotherlogic/buildserver/proto"
	pbd "github.com/brotherlogic/discovery/proto"
	pbgs "github.com/brotherlogic/gobuildslave/proto"
	"github.com/golang/protobuf/proto"
)

func (s *Server) evaluateFriends(ctx context.Context) error {
	friends, err := s.discover.getFriends(ctx)
	if err != nil {
		s.RaiseIssue(ctx, "Friend Evaluator", fmt.Sprintf("Unable to evalute friends: %v", err), false)
		return err
	}

	strFriends := strings.Split(friends, " ")
	rand.Shuffle(len(strFriends), func(i, j int) { strFriends[i], strFriends[j] = strFriends[j], strFriends[i] })

	if len(strFriends) < 2 {
		s.RaiseIssue(ctx, "Friend Evaluator", fmt.Sprintf("Unable to evaluate friends - we have less than 2: %v", strFriends), false)
		return fmt.Errorf("Short friends")
	}

	friend1 := strFriends[0]
	friend2 := strFriends[1]
	list1, err1 := s.discover.list(ctx, friend1)
	list2, err2 := s.discover.list(ctx, friend2)
	if err1 != nil || err2 != nil {
		s.RaiseIssue(ctx, "Friend Evaluator", fmt.Sprintf("%v or %v is causing an issue", err1, err2), false)
		return err1
	}

	for _, entry1 := range list1 {
		found := false
		for _, entry2 := range list2 {
			if proto.Equal(entry1, entry2) {
				found = true
			}
		}

		if !found {
			s.RaiseIssue(ctx, "Friend Evaluator", fmt.Sprintf("Mismatch in directory listing %v and then %v (%v)", list1, list2, entry1), false)
			return fmt.Errorf("Mismatch")
		}
	}

	return nil
}

func (s *Server) checkFriends(ctx context.Context) error {
	friends, err := s.discover.getFriends(ctx)
	if err != nil {
		s.RaiseIssue(ctx, "Friend Finder", fmt.Sprintf("Unable to find friends: %v", err), false)
		return err
	}

	for _, friend := range strings.Split(friends, " ") {
		rfriends, err := s.discover.getRemoteFriends(ctx, strings.Replace(strings.Replace(friend, "[", "", -1), "]", "", -1))
		if err != nil {
			s.RaiseIssue(ctx, "Friend Finder", fmt.Sprintf("Unable to get remote friends: %v", err), false)
			return err
		}
		if len(strings.Split(rfriends, " ")) != len(strings.Split(friends, " ")) {
			s.RaiseIssue(ctx, "Friend mismatch", fmt.Sprintf("For %v,%v -> %v != %v", s.Registry.Ip, friend, friends, rfriends), false)
		}
	}

	return nil
}

func (s *Server) runVersionCheck(ctx context.Context, delay time.Duration) error {
	serv, err := s.discover.ListAllServices(ctx, &pbd.ListRequest{})
	if err == nil {
		if err == nil {
			for _, service := range serv.Services.Services {
				if service.Name == "gobuildslave" {
					jobs, err := s.gobuildSlave.ListJobs(ctx, service, &pbgs.ListRequest{})
					if err == nil {
						for _, job := range jobs.Jobs {
							runningVersion := job.RunningVersion
							versions, err := s.buildServer.GetVersions(ctx, &pbbs.VersionRequest{JustLatest: true, Job: job.Job})
							if err == nil && len(versions.GetVersions()) == 0 {
								s.RaiseIssue(ctx, "Version Problem", fmt.Sprintf("%v has no version built", job.Job.Name), false)
								return nil
							}
							if len(versions.GetVersions()) > 0 {
								compiledVersion := versions.GetVersions()[0].GetVersion()
								if compiledVersion != runningVersion && len(runningVersion) > 0 {
									if _, ok := s.lastMismatchTime[service.Identifier+job.Job.Name]; !ok {
										s.lastMismatchTime[service.Identifier+job.Job.Name] = time.Now()
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

	return err
}

func (s *Server) lookForSimulBuilds(ctx context.Context) error {
	s.Log("Looking for concurrent builds")
	stats, err := s.goserver.GetStatsSingle(ctx, "buildserver")
	if err == nil {
		for _, state := range stats.States {
			if state.Key == "concurrent_builds" && state.Value > int64(4) {
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
