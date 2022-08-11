// Copyright (C) 2022 Explore.dev Unipessoal Lda. All Rights Reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package agent

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/go-github/v45/github"
	"github.com/reviewpad/action/v3/utils"
	"github.com/reviewpad/host-event-handler/handler"
	"github.com/reviewpad/reviewpad/v3"
	reviewpad_gh "github.com/reviewpad/reviewpad/v3/codehost/github"
	"github.com/reviewpad/reviewpad/v3/collector"
	"github.com/reviewpad/reviewpad/v3/engine"
)

type Env struct {
	RepoOwner    string
	RepoName     string
	Token        string
	PRNumber     int
	EventPayload interface{}
}

// reviewpad-an: critical
func runReviewpad(prNum int, e *handler.ActionEvent, mixpanelToken, filePath, fileUrl string) {
	repo := *e.Repository
	splittedRepo := strings.Split(repo, "/")
	repoOwner := splittedRepo[0]
	repoName := splittedRepo[1]
	eventPayload, err := github.ParseWebHook(*e.EventName, *e.EventPayload)

	if err != nil {
		log.Print(err)
		return
	}

	env := &Env{
		RepoOwner:    repoOwner,
		RepoName:     repoName,
		Token:        *e.Token,
		PRNumber:     prNum,
		EventPayload: eventPayload,
	}

	ctx, canc := context.WithTimeout(context.Background(), time.Minute*10)
	defer canc()

	githubClient := reviewpad_gh.NewGithubClientFromToken(ctx, env.Token)

	pullRequest, _, err := githubClient.GetPullRequest(ctx, env.RepoOwner, env.RepoName, env.PRNumber)
	if err != nil {
		log.Print(err)
		return
	}

	if pullRequest.Merged != nil && *pullRequest.Merged {
		log.Print("skip execution for merged pull requests")
		return
	}

	if err := utils.ValidateBranch(pullRequest.Base); err != nil {
		log.Fatalln(err)
	}

	if err := utils.ValidateBranch(pullRequest.Head); err != nil {
		log.Fatalln(err)
	}

	var reviewpadFileChanged bool
	var reviewpadFile *engine.ReviewpadFile

	if fileUrl != "" {
		log.Printf("using remote config file %s", fileUrl)
		branch, filePath, err := utils.ValidateUrl(fileUrl)
		if err != nil {
			log.Fatalln(err)
		}
		if reviewpadFile, err = utils.LoadReviewpadFile(ctx, githubClient, filePath, branch); err != nil {
			log.Fatalln(err)
		}
	} else {
		log.Printf("using local config file %s", filePath)
		reviewpadFileChanged, err := utils.ReviewpadFileChanged(ctx, githubClient, filePath, pullRequest)
		if err != nil {
			log.Fatalln(err)
		}

		if reviewpadFileChanged {
			if reviewpadFile, err = utils.LoadReviewpadFile(ctx, githubClient, filePath, pullRequest.Head); err != nil {
				log.Fatalln(err)
			}
		} else {
			if reviewpadFile, err = utils.LoadReviewpadFile(ctx, githubClient, filePath, pullRequest.Base); err != nil {
				log.Fatalln(err)
			}
		}
	}

	dryRun := reviewpadFileChanged

	baseRepoOwner := *pullRequest.Base.Repo.Owner.Login
	collectorClient := collector.NewCollector(mixpanelToken, baseRepoOwner)

	exitStatus, err := reviewpad.Run(ctx, githubClient, collectorClient, pullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
	if err != nil {
		if reviewpadFile.IgnoreErrors {
			log.Println(err.Error())
			return
		}

		log.Fatalln(err.Error())
	}

	if exitStatus == engine.ExitStatusFailure {
		log.Fatal("The executed program exited with code 1")
	}
}

// reviewpad-an: critical
func RunAction(githubToken, mixpanelToken, rawEvent, file, fileUrl string) {
	event, err := handler.ParseEvent(rawEvent)

	if err != nil {
		log.Printf("error parsing event: %v", err)
		return
	}

	targetEntities, err := handler.ProcessEvent(event)
	if err != nil {
		log.Printf("error processing event: %v", err)
		return
	}

	event.Token = &githubToken

	for _, targetEntity := range targetEntities {
		// TODO: add support for multiple target entities
		if targetEntity.Kind == handler.PullRequest {
			runReviewpad(targetEntity.Number, event, mixpanelToken, file, fileUrl)
		}
	}
}
