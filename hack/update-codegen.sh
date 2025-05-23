#!/usr/bin/env bash
# Copyright (C) 2025 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

 
set -o errexit
set -o nounset
set -o pipefail
 
# corresponding to go mod init <module>
MODULE=sigs.k8s.io/IOIsolation
# api package
APIS_PKG=api
# generated output package
OUTPUT_PKG=generated/ioi
# group-version such as foo:v1alpha1
GROUP_VERSION=ioi:v1
 
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
 
# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
# echo ${CODEGEN_PKG}
# bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,conversion,defaulter,client,lister,informer" \
#   ${MODULE}/${OUTPUT_PKG} ${MODULE}/${APIS_PKG} \
#   ${GROUP_VERSION} \
#   --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt \
#   --output-base "${SCRIPT_ROOT}"
#  --output-base "${SCRIPT_ROOT}/../../.." \

bash "${CODEGEN_PKG}"/generate-internal-groups.sh \
  "deepcopy,conversion,defaulter" \
  ${MODULE}/${OUTPUT_PKG} \
  ${MODULE}/${APIS_PKG} \
  ${MODULE}/${APIS_PKG} \
  "config:v1" \
  --trim-path-prefix ${MODULE} \
  --output-base "./" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

  bash "${CODEGEN_PKG}"/generate-internal-groups.sh \
  "deepcopy,client,lister,informer" \
  ${MODULE}/${OUTPUT_PKG} \
  ${MODULE}/${APIS_PKG} \
  ${MODULE}/${APIS_PKG} \
  "ioi:v1" \
  --trim-path-prefix ${MODULE} \
  --output-base "./" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt
