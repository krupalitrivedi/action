// Copyright (C) 2022 Explore.dev, Unipessoal Lda - All Rights Reserved
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package utils

import (
	"bytes"
	"context"

	"github.com/google/go-github/v42/github"
	"github.com/reviewpad/reviewpad/v3"
	"github.com/reviewpad/reviewpad/v3/engine"
)

func LoadReviewpadFile(ctx context.Context, filePath string, client *github.Client, branch *github.PullRequestBranch) (*engine.ReviewpadFile, error) {

	branchRepoOwner := *branch.Repo.Owner.Login
	branchRepoName := *branch.Repo.Name
	branchRef := *branch.Ref

	ioReader, _, err := client.Repositories.DownloadContents(ctx, branchRepoOwner, branchRepoName, filePath, &github.RepositoryContentGetOptions{
		Ref: branchRef,
	})

	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)

	buf.ReadFrom(ioReader)

	return reviewpad.Load(buf)

}
