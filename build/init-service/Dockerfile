# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.23-alpine as builder

WORKDIR /workspace
COPY . .
RUN apk add make git
RUN make patch
RUN make init-service
RUN make ioi-service

FROM alpine:3.17

ARG USER=ioi

WORKDIR /home/$USER
RUN apk add --no-cache sudo
RUN adduser -D $USER \
       && mkdir -p /etc/sudoers.d \
       && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
       && chmod 0440 /etc/sudoers.d/$USER

COPY --chown=$USER --from=builder /workspace/bin/init-service /bin/init-service
COPY --chown=$USER --from=builder /workspace/bin/ioi-service /bin/ioi-service
COPY --chown=$USER ./deployments/ioi-service.service /bin/ioi-service.service
COPY --chown=$USER ./deployments/ioi-service.toml /bin/ioi-service.toml
COPY --chown=$USER ./deployments/ioi-rdt-service.cfg /bin/ioi-rdt-service.cfg
COPY --chown=$USER ./deployments/fixedclass.yaml /bin/fixedclass.yaml
RUN chmod +x /bin/init-service
WORKDIR /bin
USER $USER
ENTRYPOINT ["sudo", "-E","./init-service"]
