# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.23-alpine as builder

WORKDIR /workspace
COPY . .
RUN apk add make
RUN make csi

FROM alpine:3.17

ARG USER=ioi

WORKDIR /home/$USER
RUN apk add --no-cache sudo
RUN adduser -D $USER \
       && mkdir -p /etc/sudoers.d \
       && echo "$USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$USER \
       && chmod 0440 /etc/sudoers.d/$USER

ARG binary=/workspace/bin/localstorage-csi
COPY --chown=$USER --from=builder ${binary} /bin/localstorage-csi
RUN chmod +x /bin/localstorage-csi
WORKDIR /bin
USER $USER
ENTRYPOINT ["sudo", "-E","./localstorage-csi"]
