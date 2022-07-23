// Copyright 2022 zola
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"

	"github.com/google/go-github/v42/github"
	"github.com/reviewpad/reviewpad/v3"
	"github.com/reviewpad/reviewpad/v3/engine"
)

var (
	ErrBranchNil               = errors.New("branch is nil")
	ErrBranchRefNil            = errors.New("branch ref is nil")
	ErrBranchRepoNil           = errors.New("branch repo is nil")
	ErrBranchRepoNameNil       = errors.New("branch repo name is nil")
	ErrBranchRepoOwnerNil      = errors.New("branch repo owner is nil")
	ErrBranchRepoOwnerLoginNil = errors.New("branch repo owner login is nil")
)

func validateBranch(branch *github.PullRequestBranch) error {

	if branch == nil {
		return ErrBranchNil
	}

	if branch.Ref == nil {
		return ErrBranchRefNil
	}

	baseRepo := branch.Repo
	if baseRepo == nil {
		return ErrBranchRepoNil
	}

	if baseRepo.Name == nil {
		return ErrBranchRepoNameNil
	}

	if baseRepo.Owner == nil {
		return ErrBranchRepoOwnerNil
	}

	if baseRepo.Owner.Login == nil {
		return ErrBranchRepoOwnerLoginNil
	}

	return nil

}

func downloadReviewPadFile(ctx context.Context, filePath string, client *github.Client, branch *github.PullRequestBranch) ([]byte, error) {

	branchRepoOwner := *branch.Repo.Owner.Login
	branchRepoName := *branch.Repo.Name
	branchRef := *branch.Ref

	ioReader, _, err := client.Repositories.DownloadContents(ctx, branchRepoOwner, branchRepoName, filePath, &github.RepositoryContentGetOptions{
		Ref: branchRef,
	})

	if err != nil {
		return []byte{}, err
	}

	return ioutil.ReadAll(ioReader)

}

func loadReviewPadFile(raw []byte) (*engine.ReviewpadFile, error) {

	buf := bytes.NewBuffer(raw)

	file, err := reviewpad.Load(buf)

	return file, err

}
