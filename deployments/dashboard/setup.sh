#!/usr/bin/env bash
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0


helm install --namespace ioi-system --create-namespace cadvisor ./cadvisor
helm install --namespace ioi-system --create-namespace prome ./prometheus

# helm delete -n ioi-system ebpf
# helm delete -n ioi-system prome
# helm delete -n ioi-system bosun
