// Copyright (C) 2022 Explore.dev, Unipessoal Lda - All Rights Reserved
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package utils

import (
	"bytes"
	"context"
	"io/ioutil"
	"strings"

	"github.com/google/go-github/v42/github"
	"github.com/reviewpad/reviewpad/v3"
	"github.com/reviewpad/reviewpad/v3/engine"
	reviewpad_utils "github.com/reviewpad/reviewpad/v3/utils"
)

func downloadReviewPadFile(ctx context.Context, filePath string, client *github.Client, branch *github.PullRequestBranch) ([]byte, error) {
	branchRepoOwner := *branch.Repo.Owner.Login
	branchRepoName := *branch.Repo.Name
	branchRef := *branch.Ref

	ioReader, _, err := client.Repositories.DownloadContents(ctx, branchRepoOwner, branchRepoName, filePath, &github.RepositoryContentGetOptions{
		Ref: branchRef,
	})

	if err != nil {
		return nil, err
	}

	return ioutil.ReadAll(ioReader)
}

func LoadReviewpadFile(ctx context.Context, filePath string, client *github.Client, branch *github.PullRequestBranch) (*engine.ReviewpadFile, error) {
	reviewpadFileContent, err := downloadReviewPadFile(ctx, filePath, client, branch)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(reviewpadFileContent)

	return reviewpad.Load(buf)
}

func ReviewpadFileChanged(ctx context.Context, filePath string, client *github.Client, pullRequest *github.PullRequest) (bool, error) {
	if *pullRequest.ChangedFiles > 50 {
		rawHeadFile, err := downloadReviewPadFile(ctx, filePath, client, pullRequest.Head)
		if err != nil {
			return false, err
		}

		rawBaseFile, err := downloadReviewPadFile(ctx, filePath, client, pullRequest.Base)
		if err != nil {
			if strings.Contains(err.Error(), "no file named") {
				return true, nil
			}
			return false, err
		}

		return !bytes.Equal(rawBaseFile, rawHeadFile), nil
	}

	branchRepoOwner := *pullRequest.Base.Repo.Owner.Login
	branchRepoName := *pullRequest.Base.Repo.Name

	files, err := reviewpad_utils.GetPullRequestFiles(ctx, client, branchRepoOwner, branchRepoName, *pullRequest.Number)
	if err != nil {
		return false, err
	}

	for _, file := range files {
		if filePath == *file.Filename {
			return true, nil
		}
	}

	return false, nil
}
