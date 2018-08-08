# Copyright 2016-2017 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

*** Settings ***
Documentation  Test 2-01 - Docker Archive
Resource  ../../resources/Util.robot
Suite Setup  Install VIC Appliance To Test Server  debug=3
Suite Teardown  Cleanup VIC Appliance Together With tmp dir

*** Keywords ***
Cleanup VIC Appliance Together With tmp dir 
    Cleanup VIC Appliance On Test Server
    Run  rm -rf /tmp/compare
    Run  rm -rf /tmp/pull
    Run  rm -rf /tmp/save

Compare Tar File Content
    [Arguments]  ${image}  ${tarA}  ${tarB}

    ${rc}  ${output}=  Run And Return Rc And Output  tar -tvf ${tarA} > /tmp/compare/${image}/a
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${output}
    ${out}=  Run  cat /tmp/compare/${image}/a
    Log  ${out}

    ${rc}  ${output}=  Run And Return Rc And Output  tar -tvf ${tarB} > /tmp/compare/${image}/b
    Should Be Equal As Integers  ${rc}  0
    Should Be Empty  ${output}
    ${out}=  Run  cat /tmp/compare/${image}/b
    Log  ${out}

    ${status}=  Get State Of Github Issue  5997
    Run Keyword If  '${status}' == 'closed'  Fail  Test 2-01-Docker-Archive.robot needs to be updated now that Issue #5997 has been resolved
    #${rc}  ${output}=  Run And Return Rc And Output  diff /tmp/compare/${image}/a /tmp/compare/${image}/b
    #Log  ${output}
    #Should Be Equal As Integers  ${rc}  0
    #Should Be Empty  ${output}

Compare Files Digest in Tar
    [Arguments]  ${image}  ${tarA}  ${tarB}

    Run  mkdir -p /tmp/compare/${image}/fileA
    Run  mkdir -p /tmp/compare/${image}/fileB
    ${rc}  ${output}=  Run And Return Rc And Output  tar -xvf ${tarA} -C /tmp/compare/${image}/fileA
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    ${rc}  ${output}=  Run And Return Rc And Output  tar -xvf ${tarB} -C /tmp/compare/${image}/fileB
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${out}=  Run  find /tmp/compare/${image}/fileA -type f
    ${files}=  Split To Lines  ${out}
    :FOR  ${fileA}  IN  @{files}
    \   ${fileB}=  Replace String  ${fileA}  /tmp/compare/${image}/fileA  /tmp/compare/${image}/fileB
    \   Log  ${fileB}
    \   ${rc}  ${output}=  Run And Return Rc And Output  sha256sum ${fileA}
    \   Should Be Equal As Integers  ${rc}  0
    \   ${digestA}=  Split String  ${output}
    \   ${rc}  ${output}=  Run And Return Rc And Output  sha256sum ${fileB}
    \   Should Be Equal As Integers  ${rc}  0
    \   ${digestB}=  Split String  ${output}
    \   Should Be Equal As Strings  @{digestA}[0]  @{digestB}[0]

Archive Download
    [Arguments]  ${image}

    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull ${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${rc}  ${output}=  Run And Return Rc And Output  bin/imagec -insecure-skip-verify -reference ${image} -destination /tmp/pull/${image} -standalone -debug -operation pull
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    ${imagestore}=  Run  govc datastore.ls %{VCH-NAME}/VIC/
    ${rc}  ${output}=  Run And Return Rc And Output  bin/imagec -insecure-skip-verify -reference ${image} -destination /tmp/save/${image} -standalone -debug -operation save -host %{VCH-IP}:2380 -image-store ${imagestore}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

Compare Images 
    [Arguments]  ${image}

    ${out}=  Run  find /tmp/pull/${image} -name *.tar
    ${files}=  Split To Lines  ${out}
    Run  mkdir -p /tmp/compare/${image}
    :FOR  ${pullfile}  IN  @{files}
    \   ${pullbase}=  Run  basename ${pullfile}
    \   ${rc}  ${savefile}=  Run And Return Rc And Output  find /tmp/save/${image} -name ${pullbase}
    \   Should Be Equal As Integers  ${rc}  0
    \   Compare Tar File Content  ${image}  ${pullfile}  ${savefile}
    \   Compare Files Digest in Tar  ${image}  ${pullfile}  ${savefile}
# TODO: compare tar file digest here

*** Test Cases *** 
Compare Busybox Image Archive
    Archive Download  ${busybox}
    Compare Images  ${busybox}

Compare Ubuntu Image Archive
    Archive Download  ${ubuntu}
    Compare Images  ${ubuntu}

Compare Postgres Image Archive
    ${status}=  Get State Of Github Issue  6059
    Run Keyword If  '${status}' == 'closed'  Fail  Test 2-01-Docker-Archive.robot needs to be updated now that Issue #6059 has been resolved
    #Archive Download  postgres:9.4
    #Compare Images  postgres:9.4
