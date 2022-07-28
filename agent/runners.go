// Copyright (C) 2022 Explore.dev Unipessoal Lda. All Rights Reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package agent

import (
	"context"
	"log"
	"strings"

	atlas "github.com/explore-dev/atlas-common/go/api/services"
	"github.com/google/go-github/v42/github"
	"github.com/reviewpad/action/v3/utils"
	"github.com/reviewpad/host-event-handler/handler"
	reviewpad_premium "github.com/reviewpad/reviewpad-premium/v3"
	"github.com/reviewpad/reviewpad/v3"
	"github.com/reviewpad/reviewpad/v3/collector"
	"github.com/reviewpad/reviewpad/v3/engine"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

var MixpanelToken string

type Env struct {
	RepoOwner        string
	RepoName         string
	Token            string
	PRNumber         int
	SemanticEndpoint string
	EventPayload     interface{}
}

// reviewpad-an: critical
func runReviewpad(prNum int, e *handler.ActionEvent, semanticEndpoint string, filePath string) {
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
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: env.Token})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	clientGQL := githubv4.NewClient(tc)

	pullRequest, _, err := client.PullRequests.Get(ctx, env.RepoOwner, env.RepoName, env.PRNumber)
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

	reviewpadFileChanged, err := utils.ReviewpadFileChanged(ctx, filePath, client, pullRequest)
	if err != nil {
		log.Fatalln(err)
	}

	var reviewpadFile *engine.ReviewpadFile

	if reviewpadFileChanged {
		if reviewpadFile, err = utils.LoadReviewpadFile(ctx, filePath, client, pullRequest.Head); err != nil {
			log.Fatalln(err)
		}
	} else {
		if reviewpadFile, err = utils.LoadReviewpadFile(ctx, filePath, client, pullRequest.Base); err != nil {
			log.Fatalln(err)
		}
	}

	dryRun := reviewpadFileChanged

	baseRepoOwner := *pullRequest.Base.Repo.Owner.Login
	collectorClient := collector.NewCollector(MixpanelToken, baseRepoOwner)

	switch reviewpadFile.Edition {
	case engine.PROFESSIONAL_EDITION:
		_, err = runReviewpadPremium(ctx, env, client, clientGQL, collectorClient, pullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
	default:
		_, err = reviewpad.Run(ctx, client, clientGQL, collectorClient, pullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
	}

	if err != nil {
		if reviewpadFile.IgnoreErrors {
			log.Println(err.Error())
			return
		}

		log.Fatalln(err.Error())
	}

}

// reviewpad-an: critical
func runReviewpadPremium(
	ctx context.Context,
	env *Env,
	client *github.Client,
	clientGQL *githubv4.Client,
	collector collector.Collector,
	ghPullRequest *github.PullRequest,
	eventPayload interface{},
	reviewpadFile *engine.ReviewpadFile,
	dryRun bool,
	reviewpadFileChanged bool,
) (*engine.Program, error) {
	defaultOptions := grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(419430400))
	semanticConnection, err := grpc.Dial(env.SemanticEndpoint, grpc.WithInsecure(), defaultOptions)
	if err != nil {
		log.Fatalf("failed to dial semantic service: %v", err)
	}
	defer semanticConnection.Close()
	semanticClient := atlas.NewSemanticClient(semanticConnection)

	return reviewpad_premium.Run(ctx, client, clientGQL, collector, semanticClient, ghPullRequest, eventPayload, reviewpadFile, dryRun, reviewpadFileChanged)
}

// reviewpad-an: critical
func RunAction(semanticEndpoint, rawEvent, token, file string) {
	event, err := handler.ParseEvent(rawEvent)
	if err != nil {
		log.Print(err)
		return
	}

	prs := handler.ProcessEvent(event)

	event.Token = &token

	for _, pr := range prs {
		runReviewpad(pr, event, semanticEndpoint, file)
	}
}
