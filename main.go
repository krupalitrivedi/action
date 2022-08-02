// Copyright (C) 2022 Explore.dev Unipessoal Lda. All Rights Reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"os"

	"github.com/reviewpad/action/v3/agent"
)

var (
	semanticEndpoint,
	rawEvent,
	file,
	gitHubToken,
	MixpanelToken string
)

func init() {
	semanticEndpoint = os.Getenv("SEMANTIC_ENDPOINT")
	if semanticEndpoint == "" {
		log.Fatal("missing SEMANTIC_ENDPOINT")
	}

	rawEvent := os.Getenv("INPUT_EVENT")
	if rawEvent == "" {
		log.Fatal("missing variable INPUT_EVENT")
	}

	file := os.Getenv("INPUT_FILE")
	if file == "" {
		log.Fatal("missing variable INPUT_FILE")
	}

	gitHubToken := os.Getenv("INPUT_TOKEN")
	if gitHubToken == "" {
		log.Fatal("missing variable INPUT_TOKEN")
	}
}

func main() {
	agent.RunAction(semanticEndpoint, gitHubToken, MixpanelToken, rawEvent, file)
}
