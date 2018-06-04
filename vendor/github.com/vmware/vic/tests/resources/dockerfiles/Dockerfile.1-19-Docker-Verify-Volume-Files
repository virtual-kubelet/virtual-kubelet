# Copyright 2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

# docker build --no-cache -t jakedsouza/group-1-19-docker-verify-volume-files:1.0 -f Dockerfile.1-19-Docker-Verify-Volume-Files .
# docker push jakedsouza/group-1-19-docker-verify-volume-files:1.0

FROM alpine:latest

RUN mkdir -p /etc/example/thisshouldexist
RUN echo "TestFile" >> /etc/example/testfile.txt

VOLUME ["/etc/example"]
