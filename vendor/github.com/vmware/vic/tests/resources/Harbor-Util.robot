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
Documentation  This resource provides any keywords related to the Harbor private registry appliance
Library  Selenium2Library
Library  OperatingSystem

*** Variables ***
${HARBOR_SHORT_VERSION}  0.5.0
${HARBOR_VERSION}  harbor_0.5.0-9e4c90e
${MY_PROJECTS_TABLE}  harbor-app > harbor-shell > clr-main-container > div > div > project > div > div > list-project
${MY_MEMBERS_TABLE}  body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(2) > clr-datagrid > div > div > div > div.datagrid-body
${harbor_cert}  getcert
${ova_harbor_admin_password}  harbor-admin-passwd

*** Keywords ***
Secret Install Harbor To Test Server
    [Tags]  secret
    [Arguments]  ${name}  ${protocol}  ${verify}  ${host}  ${datastore}  ${network}
    ${out}=  Run  ovftool --noSSLVerify --acceptAllEulas --datastore=${datastore} --name=${name} --net:"Network 1"='${network}' --diskMode=thin --powerOn --X:waitForIp --X:injectOvfEnv --X:enableHiddenProperties --prop:root_pwd=%{TEST_PASSWORD} --prop:harbor_admin_password=%{TEST_PASSWORD} --prop:db_password=%{TEST_PASSWORD} --prop:auth_mode=db_auth --prop:verify_remote_cert=${verify} --prop:protocol=${protocol} ${HARBOR_VERSION}.ova 'vi://%{TEST_USERNAME}:%{TEST_PASSWORD}@${host}'
    [Return]  ${out}

Install Harbor To Test Server
    [Arguments]  ${name}=harbor  ${protocol}=http  ${verify}=off  ${host}=%{TEST_URL_ARRAY}  ${datastore}=%{TEST_DATASTORE}  ${network}=VM Network
    Log To Console  \nFetching harbor ova...
    ${status}  ${message}=  Run Keyword And Ignore Error  OperatingSystem.File Should Exist  ${HARBOR_VERSION}.ova
    Run Keyword If  '${status}' == 'FAIL'  Download Harbor ova
    ${status}  ${message}=  Run Keyword And Ignore Error  Environment Variable Should Be Set  DRONE_BUILD_NUMBER
    Run Keyword If  '${status}' == 'FAIL'  Set Environment Variable  DRONE_BUILD_NUMBER  0
    @{URLs}=  Split String  %{TEST_URL_ARRAY}
    ${len}=  Get Length  ${URLs}
    ${IDX}=  Evaluate  %{DRONE_BUILD_NUMBER} \% ${len}

    ${rc}  ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'ESXi'  Set Suite Variable  ${host}  @{URLs}[${IDX}]
    ${rc}  ${output}=  Run Keyword If  '%{HOST_TYPE}' == 'VC'  Set Suite Variable  ${host}  @{URLs}[${IDX}]%{TEST_DATACENTER}/host/%{TEST_RESOURCE}

    Log To Console  \nDeploying ova...
    ${out}=  Secret Install Harbor To Test Server  ${name}  ${protocol}  ${verify}  ${host}  ${datastore}  ${network}
    Log  ${out}
    Should Contain  ${out}  Received IP address:
    Should Not Contain  ${out}  None

    ${out}=  Split To Lines  ${out}
    :FOR  ${line}  IN  @{out}
    \   ${status}=  Run Keyword And Return Status  Should Contain  ${line}  Received IP address:
    \   ${ip}=  Run Keyword If  ${status}  Fetch From Right  ${line}  ${SPACE}
    \   Run Keyword If  ${status}  Set Environment Variable  HARBOR-IP  ${ip}
    \   Exit For Loop If  ${status}

    Log To Console  Waiting for Harbor to Come Up...
    :FOR  ${i}  IN RANGE  20
    \  ${out}=  Run  curl -k ${protocol}://%{HARBOR-IP}
    \  Log  ${out}
    \  ${status}=  Run Keyword And Return Status  Should Not Contain Any  ${out}  502 Bad Gateway  Connection refused  Connection timed out
    \  ${status}=  Run Keyword If  ${status}  Run Keyword And Return Status  Should Contain  ${out}  <title>Harbor</title>
    \  Return From Keyword If  ${status}  %{HARBOR-IP}
    \  Sleep  30s
    Fail  Harbor failed to come up properly!
    [Return]  %{HARBOR-IP}

Restart Docker With Insecure Registry Option
    # Requires you to edit /etc/systemd/system/docker.service.d/overlay.conf or docker.conf to be:
    # ExecStart=/bin/bash -c "usr/bin/docker daemon -H fd:// -s overlay $DOCKER_OPTS --insecure-registry='cat /tmp/harbor'"
    # Requires to be run as root
    ${out}=  Run  systemctl daemon-reload
    ${out}=  Run  systemctl restart docker
    Log  ${out}

Install Harbor Self Signed Cert
    # Need to provide permissions to /etc/docker folder for your user (sudo chmod -R 777 /etc/docker)
    ${out}=  Run  wget --tries=10 --connect-timeout=10 --auth-no-challenge --no-check-certificate --user admin --password ${ova_harbor_admin_password} https://%{HARBOR_IP}/api/systeminfo/getcert
    Log  ${out}
    ${out}=  Run  mkdir -p /etc/docker/certs.d/%{HARBOR_IP}
    Move File  getcert  /etc/docker/certs.d/%{HARBOR_IP}/ca.crt
    ${out}=  Run  systemctl daemon-reload
    ${out}=  Run  systemctl restart docker

Download Harbor ova
    ${rc}  ${output}=   Run And Return Rc And Output  wget https://github.com/vmware/harbor/releases/download/${HARBOR_SHORT_VERSION}/${HARBOR_VERSION}.ova
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Log To Console    Harbor OVA download successful

Log Into Harbor
    [Arguments]  ${user}=%{TEST_USERNAME}  ${pw}=%{TEST_PASSWORD}
    Maximize Browser Window
    Input Text  login_username  ${user}
    Input Text  login_password  ${pw}
    Click button  LOG IN
    Wait Until Page Contains  Projects
    Wait Until Keyword Succeeds  5x  1  Page Should Contain Element  xpath=/html/body/harbor-app/harbor-shell/clr-main-container[@class='main-container']/navigator/clr-header[@class='header-5 header']/div[@class='header-actions']/clr-dropdown[@class='dropdown bottom-right']/button[@class='nav-text dropdown-toggle']/span[contains(., '${user}')]

Create A New Project
    [Arguments]  ${name}  ${public}=${true}
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > nav > section > a.nav-link.active
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > nav > section > a.nav-link.active
    Click Element  css=body > harbor-app > harbor-shell > clr-main-container > div > nav > section > a.nav-link.active
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project > div > div > div:nth-child(2) > div.option-left > button
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project > div > div > div:nth-child(2) > div.option-left > button
    Click Element  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project > div > div > div:nth-child(2) > div.option-left > button
    Wait Until Page Contains  New Project
    #Wait Until Element Is Visible  name="Project Name"
    #Wait Until Element Is Enabled  name="Project Name"
    Input Text  create_project_name  ${name}
    #Wait Until Element Is Visible  css=body > div.container-fluid.container-fluid-custom.ng-scope > div > div > div > add-project > div > form > div > div.col-xs-10.col-md-10 > div:nth-child(2) > label > input
    #Wait Until Element Is Enabled  css=body > div.container-fluid.container-fluid-custom.ng-scope > div > div > div > add-project > div > form > div > div.col-xs-10.col-md-10 > div:nth-child(2) > label > input
    Sleep  1
    Run Keyword If  ${public}  Select Checkbox  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project > div > div > div:nth-child(2) > div.option-left > create-project > clr-modal > div > div.modal-dialog > div > div.modal-body > form > section > div:nth-child(2) > div > label
    Click Button  OK
    Wait Until Keyword Succeeds  5x  1  Table Should Contain  css=${MY_PROJECTS_TABLE}  ${name}

Create A New User
    [Arguments]  ${name}  ${email}  ${fullName}  ${password}  ${comments}  
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > nav > section > section > ul > li:nth-child(1) > a
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > nav > section > section > ul > li:nth-child(1) > a
    Click Element  css=body > harbor-app > harbor-shell > clr-main-container > div > nav > section > section > ul > li:nth-child(1) > a
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > div > harbor-user > div > div > div.action-panel-pos > span:nth-child(1) > button
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > div > harbor-user > div > div > div.action-panel-pos > span:nth-child(1) > button
    Click Element  css=body > harbor-app > harbor-shell > clr-main-container > div > div > harbor-user > div > div > div.action-panel-pos > span:nth-child(1) > button

    Wait Until Element Is Visible  username
    Wait Until Element Is Enabled  username
    Wait Until Element Is Visible  email
    Wait Until Element Is Enabled  email
    Wait Until Element Is Visible  realname
    Wait Until Element Is Enabled  realname
    Wait Until Element Is Visible  newPassword
    Wait Until Element Is Enabled  newPassword
    Wait Until Element Is Visible  confirmPassword
    Wait Until Element Is Enabled  confirmPassword
    Wait Until Element Is Visible  comment
    Wait Until Element Is Enabled  comment

    Input Text  username  ${name}
    Input Text  email  ${email}
    Input Text  realname  ${fullName}
    Input Text  newPassword  ${password}
    Input Text  confirmPassword  ${password}
    Input Text  comment  ${comments}

    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > div > harbor-user > div > div > new-user-modal > clr-modal > div > div.modal-dialog > div > div.modal-footer > button.btn.btn-primary
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > div > harbor-user > div > div > new-user-modal > clr-modal > div > div.modal-dialog > div > div.modal-footer > button.btn.btn-primary
    Click Button  css=body > harbor-app > harbor-shell > clr-main-container > div > div > harbor-user > div > div > new-user-modal > clr-modal > div > div.modal-dialog > div > div.modal-footer > button.btn.btn-primary

    Wait Until Page Contains  New user created successfully.
    Sleep  3

Toggle Admin Priviledges For User
    [Arguments]  ${user}
    Wait Until Element Is Visible  //a[@tag='admin_option']
    Wait Until Element Is Enabled  //a[@tag='admin_option']
    Click Link  Admin Options
    Table Should Contain  css=body > div.container-fluid.container-fluid-custom.ng-scope > div > div > div > list-user > div > div > div.pane > div.sub-pane > div.table-body-container > table  ${user}
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()-1]/toggle-admin/button[not(contains(@class, 'ng-hide'))]
    Wait Until Element Is Enabled  xpath=//td[text()='test-user']/../td[last()-1]/toggle-admin/button[not(contains(@class, 'ng-hide'))]
    ${oldPublicity}=  Get Text  xpath=//td[text()='test-user']/../td[last()-1]/toggle-admin/button[not(contains(@class, 'ng-hide'))]
    Click Button  xpath=//td[text()='test-user']/../td[last()-1]/toggle-admin/button[not(contains(@class, 'ng-hide'))]
    Sleep  1
    Log to Console  show result
    ${newPublicity}=  Get Text  xpath=//td[text()='test-user']/../td[last()-1]/toggle-admin/button[not(contains(@class, 'ng-hide'))]
    Should Not Be Equal  ${oldPublicity}  ${newPublicity}
    [return]  ${newPublicity}

Delete A User
    [Arguments]  ${user}
    Wait Until Element Is Visible  //a[@tag='admin_option']
    Wait Until Element Is Enabled  //a[@tag='admin_option']
    Click Link  Admin Options
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a
    Wait Until Element Is Enabled  xpath=//td[text()='${user}']/../td[last()]/a
    Click Link  xpath=//td[text()='${user}']/../td[last()]/a
    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  Are you sure to delete the user "${user}" ?
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Element Should Not Contain  css=div.table-body-container > table  ${user}

Search For A User
    [Arguments]  ${keyword}
    Wait Until Element Is Visible  //a[@tag='admin_option']
    Wait Until Element Is Enabled  //a[@tag='admin_option']
    Click Link  Admin Options
    Wait Until Element Is Visible  txtSearchInput
    Wait Until Element Is Enabled  txtSearchInput
    Input Text  txtSearchInput  ${keyword}
    Wait Until Element Is Visible  css=span.input-group-btn > button
    Wait Until Element Is Enabled  css=span.input-group-btn > button
    Click Button  css=span.input-group-btn > button
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Table Should Contain  css=div.table-body-container > table  ${keyword}
    # check all result contains the search keyword
    Wait Until Element Is Visible  xpath=//tbody/tr/td[1]
    Wait Until Element Is Enabled  xpath=//tbody/tr/td[1]
    ${rowNum}=  Get Matching Xpath Count  xpath=//tbody/tr/td[1]
    ${names}=  Create List
    ${realRowNum}=  Evaluate  ${rowNum} + 1
    :FOR  ${idx}  IN RANGE  1  ${realRowNum}
    \  ${searchName}=  Get Text  xpath=//tbody/tr[${idx}]/td[1]
    \  Should Match Regexp  ${searchName}  .*${keyword}.*
    \  Append To List  ${names}  ${searchName}
    [return]  ${names}

Change User Information
    [Arguments]  ${email}  ${fullName}  ${comments}
    Wait Until Element Is Visible  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Wait Until Element Is Enabled  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Click Link  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Wait Until Element Is Visible  xpath=//a[contains(., 'Account Settings')]
    Wait Until Element Is Enabled  xpath=//a[contains(., 'Account Settings')]
    Click Link  xpath=//a[contains(., 'Account Settings')]
    Wait Until Element Is Visible  email
    Wait Until Element Is Enabled  email
    Wait Until Element Is Visible  fullName
    Wait Until Element Is Enabled  fullName
    Wait Until Element Is Visible  comments
    Wait Until Element Is Enabled  comments
    Input Text  email  ${email}
    Input Text  fullName  ${fullName}
    Input Text  comments  ${comments}

    Wait Until Element Is Visible  xpath=//input[@value='Save']
    Wait Until Element Is Enabled  xpath=//input[@value='Save']
    Click Element  xpath=//input[@value='Save']

    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  User profile has been changed successfully.
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Wait Until Page Contains  Summary
    Wait Until Page Contains  My Projects:

Change User Password
    [Arguments]  ${password}  ${newPassword}
    Wait Until Element Is Visible  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Wait Until Element Is Enabled  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Click Link  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Wait Until Element Is Visible  xpath=//a[contains(., 'Account Settings')]
    Wait Until Element Is Enabled  xpath=//a[contains(., 'Account Settings')]
    Click Link  xpath=//a[contains(., 'Account Settings')]
    Wait Until Element Is Visible  toggleChangePassword
    Wait Until Element Is Enabled  toggleChangePassword
    Click Link  toggleChangePassword
    Wait Until Element Is Visible  oldPassword
    Wait Until Element Is Enabled  oldPassword
    Wait Until Element Is Visible  password
    Wait Until Element Is Enabled  password
    Wait Until Element Is Visible  confirmPassword
    Wait Until Element Is Enabled  confirmPassword
    Input Text  oldPassword  ${password}
    Input Text  password  ${newPassword}
    Input Text  confirmPassword  ${newPassword}

    Wait Until Element Is Visible  xpath=//input[@value='Save']
    Wait Until Element Is Enabled  xpath=//input[@value='Save']
    Click Element  xpath=//input[@value='Save']
    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  Password has been changed successfully.
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Wait Until Page Contains  Summary
    Wait Until Page Contains  My Projects:

Logout Harbor
    Wait Until Element Is Visible  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Wait Until Element Is Enabled  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Click Link  css=#bs-harbor-navbar-collapse-1 > optional-menu > div > a
    Wait Until Element Is Visible  xpath=//a[contains(., 'Log Out')]
    Wait Until Element Is Enabled  xpath=//a[contains(., 'Log Out')]
    Click Link  xpath=//a[contains(., 'Log Out')]
    Wait Until Keyword Succeeds  5x  1  Page Should Contain Element  xpath=//h4[text()='Login Now']

Sign up
    [Arguments]  ${name}  ${email}  ${fullName}  ${password}  ${comments}
    Wait Until Element Is Visible  xpath=//button[text()='Sign Up']
    Wait Until Element Is Enabled  xpath=//button[text()='Sign Up']
    Click Button  xpath=//button[text()='Sign Up']
    Wait Until Keyword Succeeds  5x  1  Page Should Contain Element  xpath=//button[text()='Sign Up']
    Wait Until Element Is Visible  username
    Wait Until Element Is Enabled  username
    Wait Until Element Is Visible  email
    Wait Until Element Is Enabled  email
    Wait Until Element Is Visible  fullName
    Wait Until Element Is Enabled  fullName
    Wait Until Element Is Visible  password
    Wait Until Element Is Enabled  password
    Wait Until Element Is Visible  confirmPassword
    Wait Until Element Is Enabled  confirmPassword
    Wait Until Element Is Visible  comments
    Wait Until Element Is Enabled  comments

    Input Text  username  ${name}
    Input Text  email  ${email}
    Input Text  fullName  ${fullName}
    Input Text  password  ${password}
    Input Text  confirmPassword  ${password}
    Input Text  comments  ${comments}

    Wait Until Element Is Visible  xpath=//button[text()='Sign Up']
    Wait Until Element Is Enabled  xpath=//button[text()='Sign Up']
    Click Button  xpath=//button[text()='Sign Up']
    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  Signed up successfully.
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)

    Wait Until Keyword Succeeds  5x  1  Page Should Contain Element  xpath=//h4[text()='Login Now']

Add A User To A Project
    # role should be one of the strings : 'Project Admin'/'Developer'/'Guest'
    [Arguments]  ${user}  ${project}  ${role}
    Click Link  Projects
    Wait Until Keyword Succeeds  5x  1  Table Should Contain  css=${MY_PROJECTS_TABLE}  ${project}
    Wait Until Element Is Visible  xpath=/html/body/harbor-app/harbor-shell/clr-main-container/div/div/project/div/div/list-project/clr-datagrid/div/div/div/div[2]/clr-dg-row[2]/clr-dg-cell[2]/a[text()='${project}']
    Wait Until Element Is Enabled  xpath=/html/body/harbor-app/harbor-shell/clr-main-container/div/div/project/div/div/list-project/clr-datagrid/div/div/div/div[2]/clr-dg-row[2]/clr-dg-cell[2]/a[text()='${project}']
    Click Link  xpath=/html/body/harbor-app/harbor-shell/clr-main-container/div/div/project/div/div/list-project/clr-datagrid/div/div/div/div[2]/clr-dg-row[2]/clr-dg-cell[2]/a[text()='${project}']
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > nav > ul > li:nth-child(2) > a
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > nav > ul > li:nth-child(2) > a
    Click Link  Members
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(1) > div > div.flex-xs-middle.option-left > button
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(1) > div > div.flex-xs-middle.option-left > button
    Click Button  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(1) > div > div.flex-xs-middle.option-left > button
    Wait Until Element Is Visible  member_name
    Wait Until Element Is Enabled  member_name
    Input Text  member_name  ${user}
    Wait Until Element Is Visible  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(1) > div > div.flex-xs-middle.option-left > add-member > clr-modal > div > div.modal-dialog > div > div.modal-footer > button.btn.btn-primary
    Wait Until Element Is Enabled  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(1) > div > div.flex-xs-middle.option-left > add-member > clr-modal > div > div.modal-dialog > div > div.modal-footer > button.btn.btn-primary
    Click Button  css=body > harbor-app > harbor-shell > clr-main-container > div > div > project-detail > ng-component > div > div:nth-child(1) > div > div.flex-xs-middle.option-left > add-member > clr-modal > div > div.modal-dialog > div > div.modal-footer > button.btn.btn-primary
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Table Should Contain  css=${MY_MEMBERS_TABLE}  ${user}

Remove A User From A Project
    [Arguments]  ${user}  ${project}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Table Should Contain  css=${MY_PROJECTS_TABLE}  ${project}
    Wait Until Element Is Visible  xpath=//td/a[text()='${project}']
    Wait Until Element Is Enabled  xpath=//td/a[text()='${project}']
    Click Link  xpath=//td/a[text()='${project}']
    Wait Until Element Is Visible  xpath=//a[@tag='users']
    Wait Until Element Is Enabled  xpath=//a[@tag='users']
    Click Link  Users
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[last()]
    Wait Until Element Is Enabled  xpath=//td[text()='${user}']/../td[last()]/a[last()]
    Click Link  xpath=//td[text()='${user}']/../td[last()]/a[last()]
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Page Should Not Contain  ${user}

Change A User's Role In A Project
    [Arguments]  ${user}  ${project}  ${role}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Wait Until Element Is Visible  css=${MY_PROJECTS_TABLE}
    Table Should Contain  css=${MY_PROJECTS_TABLE}  ${project}
    Click Link  xpath=//td/a[text()='${project}']
    Wait Until Element Is Visible  xpath=//a[@tag='users']
    Wait Until Element Is Enabled  xpath=//a[@tag='users']
    Click Link  Users
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Wait Until Element Is Enabled  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[1]/span[@title='Edit']
    Click Link  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Wait Until Element Is Visible  css=select.form-control
    Wait Until Element Is Enabled  css=select.form-control
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Wait Until Element Is Enabled  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[1]/span[@title='Confirm']
    Select From List By Label  css=select  ${role}
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Wait Until Element Is Enabled  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Click Link  xpath=//td[text()='${user}']/../td[last()]/a[1]
    Sleep  1
    Wait Until Element Is Visible  xpath=//td[text()='${user}']/../td[last()]/a[1]/span[@title='Edit']
    Wait Until Keyword Succeeds  5x  1  Page Should Contain Element  //td[text()='${user}']/../td[2]/switch-role/ng-switch/span[text()='${role}']

Delete A Project
    [Arguments]  ${project}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Wait Until Element Is Visible  xpath=//td/a[text()='${project}']/../../td[last()]/a
    Wait Until Element Is Enabled  xpath=//td/a[text()='${project}']/../../td[last()]/a
    Click Link  xpath=//td/a[text()='${project}']/../../td[last()]/a
    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  Are you sure to delete the project "${project}" ?
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Element Should Not Contain  css=${MY_PROJECTS_TABLE}  ${project}

Search For A Project
    # search for the project contains the keyword, and return all result as a list
    [Arguments]  ${keyword}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Wait Until Element Is Visible  xpath=//input[@ng-model='vm.projectName']
    Wait Until Element Is Enabled  xpath=//input[@ng-model='vm.projectName']
    Input Text  xpath=//input[@ng-model='vm.projectName']  ${keyword}
    Wait Until Element Is Visible  css=span.input-group-btn > button
    Wait Until Element Is Enabled  css=span.input-group-btn > button
    Click Button  css=span.input-group-btn > button
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Table Should Contain  css=${MY_PROJECTS_TABLE}  ${keyword}
    # check all result contains the search keyword
    Wait Until Element Is Visible  xpath=//tbody/tr/td[1]
    Wait Until Element Is Enabled  xpath=//tbody/tr/td[1]
    ${rowNum}=  Get Matching Xpath Count  xpath=//tbody/tr/td[1]
    ${names}=  Create List
    ${realRowNum}=  Evaluate  ${rowNum} + 2
    :FOR  ${idx}  IN RANGE  2  ${realRowNum}
    \  ${searchName}=  Get Text  xpath=//tbody/tr[${idx}]/td[1]
    \  Should Match Regexp  ${searchName}  .*${keyword}.*
    \  Append To List  ${names}  ${searchName}
    [return]  ${names}

Delete Repository From Project
    [Arguments]  ${image}  ${project}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Table Should Contain  css=${MY_PROJECTS_TABLE}  ${project}
    Wait Until Element Is Visible  xpath=//td/a[text()='${project}']
    Wait Until Element Is Enabled  xpath=//td/a[text()='${project}']
    Click Link  xpath=//td/a[text()='${project}']
    Wait Until Element Is Visible  xpath=//a[contains(., '${project}/${image}')]/../a[last()]
    Wait Until Element Is Enabled  xpath=//a[contains(., '${project}/${image}')]/../a[last()]
    Click Link  xpath=//a[contains(., '${project}/${image}')]/../a[last()]
    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  Delete repository "${project}/${image}" now?
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Sleep  1
    Wait Until Keyword Succeeds  5x  1  Element Should Not Contain  css=div.sub-pane  ${project}/${image}

Delete Image From Project
    [Arguments]  ${image}  ${tag}  ${project}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Table Should Contain  css=${MY_PROJECTS_TABLE}  ${project}
    Wait Until Element Is Visible  xpath=//td/a[text()='${project}']
    Wait Until Element Is Enabled  xpath=//td/a[text()='${project}']
    Click Link  xpath=//td/a[text()='${project}']
    Wait Until Element Is Visible  xpath=//a[contains(., '${project}/${image}')]
    Wait Until Element Is Enabled  xpath=//a[contains(., '${project}/${image}')]
    Click Link  xpath=//a[contains(., '${project}/${image}')]
    Wait Until Element Is Visible  xpath=//a[contains(., '${project}/${image}')]/../../../div[2]/div/table/tbody/tr/td[text()='${tag}']/../td[last()]/a
    Wait Until Element Is Enabled  xpath=//a[contains(., '${project}/${image}')]/../../../div[2]/div/table/tbody/tr/td[text()='${tag}']/../td[last()]/a
    Click Link  xpath=//a[contains(., '${project}/${image}')]/../../../div[2]/div/table/tbody/tr/td[text()='${tag}']/../td[last()]/a
    Wait Until Element Is Visible  css=div.modal.fade.in > div > div > div:nth-child(2)
    Wait Until Element Contains  css=div.modal.fade.in > div > div > div:nth-child(2)  Delete tag "${tag}" now?
    Wait Until Element Is Enabled  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Click Button  css=div.modal.fade.in > div > div > div:nth-child(3) > button:nth-child(1)
    Sleep  1
    # if it is last image in this repo, the repo will be deleted
    ${imageNum}=  Get Text  xpath=//a[contains(., '${project}/${image}')]/span[2]
    Run Keyword If  '${imageNum}'=='1'  Wait Until Keyword Succeeds  5x  1  Element Should Not Contain  css=div.sub-pane  ${tag}
    ...  Else  Wait Until Keyword Succeeds  5x  1  Element Should Not Contain  xpath=//a[contains(., '${project}/${image}')]/../../../div[2]/div/table/tbody  ${tag}

Toggle Publicity On Project
    [Arguments]  ${project}
    Wait Until Element Is Visible  //a[@tag='project']
    Wait Until Element Is Enabled  //a[@tag='project']
    Click Link  Projects
    Wait Until Element Is Visible  xpath=//td/a[text()='${project}']/../../td[last()-1]/publicity-button/button
    Wait Until Element Is Enabled  xpath=//td/a[text()='${project}']/../../td[last()-1]/publicity-button/button
    ${oldPublicity}=  Get Text  xpath=//td/a[text()='${project}']/../../td[last()-1]/publicity-button/button
    Click Button  //td/a[text()='${project}']/../../td[last()-1]/publicity-button/button
    Sleep  1
    ${newPublicity}=  Get Text  xpath=//td/a[text()='${project}']/../../td[last()-1]/publicity-button/button
    Should Not Be Equal  ${oldPublicity}  ${newPublicity}
    [return]  ${newPublicity}

Go To HomePage
    Wait Until Element Is Visible  css=a.navbar-brand
    Wait Until Element Is Enabled  css=a.navbar-brand
    Click Link  css=a.navbar-brand

    Wait Until Page Contains  Summary
    Wait Until Page Contains  My Projects:

Check That VM Is Removed
    [Arguments]  ${container}
    ${rc}  ${output}=  Run And Return Rc And Output  govc ls vm
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${container}

Check That Datastore Is Cleaned
    [Arguments]  ${container}
    ${rc}  ${output}=  Run And Return Rc And Output  govc datastore.ls
    Should Be Equal As Integers  ${rc}  0
    Should Not Contain  ${output}  ${container}

Create Project And Three Users For It
    [Arguments]  ${developer}  ${developer2}  ${developerEmail}  ${developerEmail2}  ${developerFullName}  ${password}  ${userPassword}  ${comments}  ${guest}  ${developerRole}  ${guestRole}  ${project}  ${public}=${False}
    # 2 developers, 1 guest
    Log To Console  Create Three Users For Project..
    Open Browser  https://%{HARBOR_IP}/  chrome
    Log To Console  Opened
    Log Into Harbor  user=admin  pw=${password}

    Create A New Project  name=${project}  public=${False}
    Log To Console  Create a New User..
    Create A New User  name=${developer}  email=${developerEmail}  fullName=${developerFullName}  password=${userPassword}  comments=${comments}
    Create A New User  name=${guest}  email=${guestEmail}  fullName=${guestFullName}  password=${userPassword}  comments=${comments}
    Create A New User  name=${developer2}  email=${developerEmail2}  fullName=${developerFullName}  password=${userPassword}  comments=${comments}

    Add A User To A Project  user=${developer}  project=${project}  role=${developerRole}
    Add A User To A Project  user=${guest}  project=${project}  role=${guestRole}
    Add A User To A Project  user=${developer2}  project=${project}  role=${developerRole}
    Log To Console  User Creation Complete..
    Close All Browsers

Basic Docker Command With Harbor
    [Arguments]  ${user}  ${password}  ${project}  ${image}  ${container_name}
    # Docker login
    Log To Console  \nRunning docker login ${user}...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} login -u ${user} -p ${password} %{HARBOR_IP}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Login Succeeded
    Should Not Contain  ${output}  Error response from daemon

    # Docker pull from dockerhub
    Log To Console  docker pull from dockerhub...
    ${rc}  ${output}=  Run And Return Rc And Output  docker pull ${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker tag image
    Log To Console  docker tag...
    ${rc}  ${output}=  Run And Return Rc And Output  docker tag ${image} %{HARBOR_IP}/${project}/${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker push image
    Log To Console  push image...
    ${rc}  ${output}=  Run And Return Rc And Output  docker push %{HARBOR_IP}/${project}/${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  digest:
    Should Contain  ${output}  latest:
    Should Not Contain  ${output}  No such image:

    # Docker delete image in local registry
    Log To Console  docker rmi...
    ${rc}  ${output}=  Run And Return Rc And Output  docker rmi -f %{HARBOR_IP}/${project}/${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Untagged

    # Docker pull from harbor using VCH
    Log To Console  docker pull from harbor using VCH...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} pull %{HARBOR_IP}/${project}/${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker run image
    Log To Console  docker run...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} run --name ${container_name} %{HARBOR_IP}/${project}/${image} /bin/ash -c "dmesg;echo END_OF_THE_TEST" 
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  END_OF_THE_TEST

    # Docker rm container
    Log To Console  docker rm...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${container_name}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker create
    Log To Console  docker create...
    ${rc}  ${containerID}=  Run And Return Rc And Output  docker %{VCH-PARAMS} create --name ${container_name} -i %{HARBOR_IP}/${project}/${image} /bin/top
    Log  ${containerID}
    Should Be Equal As Integers  ${rc}  0

    # Docker start
    Log To Console  docker start...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container_name}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker attach
    Log To Console  Starting process Docker attach...
    Start Process  docker %{VCH-PARAMS} attach ${container_name} < /tmp/fifo  shell=True  alias=custom
    Sleep  3
    Run  echo q > /tmp/fifo
    ${ret}=  Wait For Process  custom
    Log  ${ret}
    Should Be Equal As Integers  ${ret.rc}  0
    Should Be Empty  ${ret.stdout}
    Should Be Empty  ${ret.stderr}

    # Docker start
    Log To Console  docker start...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} start ${container_name}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker stop
    Log To Console  docker stop...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} stop ${container_name}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0

    # Docker remove
    Log To Console  docker rm...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rm -f ${container_name}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Wait Until Keyword Succeeds  10x  6s  Check That VM Is Removed  ${container_name}
    Wait Until Keyword Succeeds  10x  6s  Check That Datastore Is Cleaned  ${container_name}

    # Docker delete image
    Log To Console  docker rmi...
    ${rc}  ${output}=  Run And Return Rc And Output  docker %{VCH-PARAMS} rmi -f %{HARBOR_IP}/${project}/${image}
    Log  ${output}
    Should Be Equal As Integers  ${rc}  0
    Should Contain  ${output}  Untagged

Fetch Harbor Self Signed Cert
    [Tags]  secret
    [Arguments]  ${harbor-ip}  ${user}=admin  ${password}=%{TEST_PASSWORD}
    Remove File  ${harbor_cert}
    ${rc}=  Run And Return Rc  wget -q --tries=10 --connect-timeout=10 --auth-no-challenge --no-check-certificate --user ${user} --password ${password} https://${harbor-ip}/api/systeminfo/${harbor_cert}
    Should Be Equal As Integers  ${rc}  0
    [Return]  ${harbor_cert}

# Requires vc credential for govc
Cleanup Harbor
    [Tags]  secret
    [Arguments]  ${harbor-name}  ${host}=%{TEST_URL}  ${user}=%{TEST_USERNAME}  ${password}=%{TEST_PASSWORD}
    Log To Console  \nCleanup Harbor... TEST_URL:%{TEST_URL}
    Remove File  ${HARBOR_VERSION}.ova
    Remove File  ${harbor_cert}
    ${out}=  Run Keyword And Ignore Error  Run  GOVC_URL=${host} GOVC_USERNAME=${user} GOVC_PASSWORD=${password} GOVC_INSECURE=1 govc vm.destroy ${harbor-name}
    [Return]  ${out}
