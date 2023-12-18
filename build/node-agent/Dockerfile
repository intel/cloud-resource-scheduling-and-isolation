# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

# Build the manager binary
FROM golang:1.23-alpine as builder

WORKDIR /workspace
COPY . .
RUN apk add make
RUN make node-agent

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.17

ARG USER=ioi

WORKDIR /home/$USER
RUN apk add --no-cache fio sudo


RUN adduser -D $USER \
       && mkdir -p /etc/sudoers.d \
       && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
       && chmod 0440 /etc/sudoers.d/$USER

COPY --chown=$USER --from=builder /workspace/bin/node-agent /bin/node-agent
COPY --chown=$USER --from=builder /workspace/deployments/config.toml /etc/ioi/config/

RUN chmod +x /bin/node-agent
WORKDIR /bin
USER $USER

ENTRYPOINT ["sudo", "-E","node-agent"]
