// Copyright (C) 2022 Explore.dev Unipessoal Lda. All Rights Reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package agent

import (
	"context"
	"log"
	"strings"

	atlas "github.com/explore-dev/atlas-common/go/api/services"
	"github.com/google/go-github/v45/github"
	"github.com/reviewpad/action/v3/utils"
	"github.com/reviewpad/host-event-handler/handler"
	reviewpad_premium "github.com/reviewpad/reviewpad-premium/v3"
	"github.com/reviewpad/reviewpad/v3"
	reviewpad_gh "github.com/reviewpad/reviewpad/v3/codehost/github"
	"github.com/reviewpad/reviewpad/v3/collector"
	"github.com/reviewpad/reviewpad/v3/engine"
	"google.golang.org/grpc"
)

type Env struct {
	RepoOwner        string
	RepoName         string
	Token            string
	PRNumber         int
	SemanticEndpoint string
	EventPayload     interface{}
}

// reviewpad-an: critical
func runReviewpad(prNum int, e *handler.ActionEvent, semanticEndpoint string, mixpanelToken string, filePath, fileUrl string) {
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
		RepoOwner:        repoOwner,
		RepoName:         repoName,
		Token:            *e.Token,
		PRNumber:         prNum,
		SemanticEndpoint: semanticEndpoint,
		EventPayload:     eventPayload,
	}

	ctx := context.Background()
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
		if reviewpadFile, err = utils.LoadReviewpadFile(ctx, filePath, githubClient, branch); err != nil {
			log.Fatalln(err)
		}
	} else {
		log.Printf("using local config file %s", filePath)
		reviewpadFileChanged, err := utils.ReviewpadFileChanged(ctx, filePath, githubClient, pullRequest)
		if err != nil {
			log.Fatalln(err)
		}

		if reviewpadFileChanged {
			if reviewpadFile, err = utils.LoadReviewpadFile(ctx, filePath, githubClient, pullRequest.Head); err != nil {
				log.Fatalln(err)
			}
		} else {
			if reviewpadFile, err = utils.LoadReviewpadFile(ctx, filePath, githubClient, pullRequest.Base); err != nil {
				log.Fatalln(err)
			}
		}
	}

	dryRun := reviewpadFileChanged

	baseRepoOwner := *pullRequest.Base.Repo.Owner.Login
	collectorClient := collector.NewCollector(mixpanelToken, baseRepoOwner)

	var exitStatus engine.ExitStatus
	switch reviewpadFile.Edition {
	case engine.PROFESSIONAL_EDITION:
		exitStatus, err = runReviewpadPremium(ctx, env, githubClient, collectorClient, pullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
	default:
		exitStatus, err = reviewpad.Run(ctx, githubClient, collectorClient, pullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
	}

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
func runReviewpadPremium(
	ctx context.Context,
	env *Env,
	githubClient *reviewpad_gh.GithubClient,
	collector collector.Collector,
	ghPullRequest *github.PullRequest,
	eventPayload interface{},
	reviewpadFile *engine.ReviewpadFile,
	dryRun bool,
	reviewpadFileChanged bool,
) (engine.ExitStatus, error) {
	defaultOptions := grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(419430400))
	semanticConnection, err := grpc.Dial(env.SemanticEndpoint, grpc.WithInsecure(), defaultOptions)
	if err != nil {
		log.Fatalf("failed to dial semantic service: %v", err)
	}
	defer semanticConnection.Close()
	semanticClient := atlas.NewSemanticClient(semanticConnection)

	return reviewpad_premium.Run(ctx, githubClient, collector, semanticClient, ghPullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
}

// reviewpad-an: critical
func RunAction(semanticEndpoint, gitHubToken, mixpanelToken, rawEvent, file, fileUrl string) {
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

	event.Token = &gitHubToken

	for _, targetEntity := range targetEntities {
		switch targetEntity.Kind {
		case "pull_request":
			runReviewpad(targetEntity.Number, event, semanticEndpoint, mixpanelToken, file, fileUrl)
		default:
			log.Printf("unsupported target entity kind: %s", targetEntity.Kind)
		}
	}
}
