# Copyright 2024 Intel Corporation
# 
# Licensed under the Apache License, Version 2.0 (the License);
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#    http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.21-alpine as builder

WORKDIR /workspace
COPY . .
RUN apk add make
RUN make iodriver

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM alpine:3.17

COPY --from=builder /workspace/bin/iodriver /bin/iodriver

RUN chmod +x /bin/iodriver
WORKDIR /bin

CMD ["iodriver"]