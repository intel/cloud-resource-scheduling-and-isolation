# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# Build the manager binary
FROM golang:1.23-alpine as builder

WORKDIR /workspace
COPY . .
RUN apk add make
RUN make aggregator

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.17

ARG USER=ioi

WORKDIR /home/$USER
RUN apk add --no-cache sudo


RUN adduser -D $USER
COPY --chown=$USER --from=builder /workspace/bin/aggregator /bin/aggregator
RUN chmod +x /bin/aggregator && mkdir -p /etc/$USER/control/pki && chown $USER:$USER -R /etc/$USER/control/pki

WORKDIR /bin
USER $USER

ENTRYPOINT ["./aggregator"]
