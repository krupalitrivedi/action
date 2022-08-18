# Copyright (C) 2022 Explore.dev, Unipessoal Lda - All Rights Reserved
# Use of this source code is governed by a license that can be
# found in the LICENSE file.

FROM exploredev/reviewpad:semanticservice-v1.14 as semanticservice

FROM golang:1.18.5-alpine AS build

ARG mixpanelToken

WORKDIR /service

# Download the dependencies as a separate, cacheable step
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

# Build the project
COPY . .
RUN go build -ldflags "-X main.MixpanelToken=$mixpanelToken"

# Final image
FROM gcr.io/distroless/cc:debug

SHELL ["/busybox/sh", "-c"]

ENV ATLAS_SEMANTIC_PORT="0.0.0.0:3006"
ENV INPUT_SEMANTIC_SERVICE="0.0.0.0:3006"

WORKDIR /app

# Semantic service
COPY --from=semanticservice /semantic-server /app/semantic-server

COPY --from=build /service/action /app/action

COPY ./run.sh .
RUN chmod +x /app/run.sh

ENTRYPOINT ["sh","/app/run.sh"]
