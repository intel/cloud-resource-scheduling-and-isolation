# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.23-alpine AS builder

WORKDIR /go/src/ioi.intel.com/scheduler-plugins
COPY . .
RUN apk add make
RUN make scheduler

FROM alpine:3.16
COPY --from=builder /go/src/ioi.intel.com/scheduler-plugins/bin/scheduler /bin/kube-scheduler
WORKDIR /bin
ENTRYPOINT ["kube-scheduler"]
