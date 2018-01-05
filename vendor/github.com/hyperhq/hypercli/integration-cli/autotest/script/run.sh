#!/bin/bash

export HYPER_CONFIG=~/.hyper
export DOCKER_REMOTE_DAEMON=1
export DOCKER_CERT_PATH=fixtures/hyper_ssl
export DOCKER_TLS_VERIFY=

#check hyper credentrial
if [[ "${ACCESS_KEY}" == "" ]] || [[ "${SECRET_KEY}" == "" ]];then
    echo "Error: Please set ACCESS_KEY and SECRET_KEY"
    exit 1
fi


if [ "${TARGET_REGION}" = "eu-central-1" ]
then
    #test eu1 with zl2 container
    export TARGET_NAME="eu1"
    export REGION="eu-central-1"
    export DOCKER_HOST="tcp://${REGION}.hyper.sh:443"
elif [ "${TARGET_REGION}" = "us-west-1" ]
then
    #test zl2 with eu1 container
    export TARGET_NAME="zl2"
    export REGION="us-west-1"
    export DOCKER_HOST="tcp://${REGION}.hyper.sh:443"
elif [ "${TARGET_REGION}" = "RegionOne" ]
then
    #test packet with zl2 container
    export TARGET_NAME="pkt"
    export REGION="RegionOne"
    export DOCKER_HOST="tcp://147.75.195.39:6443"
else
    echo "unknow TARGET_REGION:${TARGET_REGION}"
    exit 1
fi
TITLE="*hypercli integration auto-test for \`${TARGET_NAME}\`* - ${BEGIN_TIME} *\`${TEST_CASE_REG}\`*"


# job url
JOB_URL="http://ci.hypercontainer.io:8080/job/${JOB_NAME}/${BUILD_NUMBER}/console"
echo "JOB_URL: ${JOB_URL}"

# branch url
PR_PRE=$(expr substr ${BRANCH} 1 1)
if [ "$PR_PRE" = "#" ]
then
    PR_NUMBER=$(echo ${BRANCH} | awk '{print substr($1,2)}')
    echo "========== Task: test PR ${PR_NUMBER} =========="
    BRANCH_URL="https://github.com/hyperhq/hypercli/pull/${PR_NUMBER}/commits"
else
    echo "========== Task: test BRANCH ${BRANCH} =========="
    BRANCH_URL="https://github.com/hyperhq/hypercli/commits/${BRANCH}"
fi
echo "BRANCH_URL: ${BRANCH_URL}"

#ensure config for hyper cli
mkdir -p ${HYPER_CONFIG}
cat > ${HYPER_CONFIG}/config.json <<EOF
{
    "clouds": {
        "${DOCKER_HOST}": {
            "accesskey": "${ACCESS_KEY}",
            "secretkey": "${SECRET_KEY}",
            "region": "${REGION}"
        },
        "tcp://*.hyper.sh:443": {
            "accesskey": "${ACCESS_KEY}",
            "secretkey": "${SECRET_KEY}",
            "region": "${REGION}"
        }
    }
}
EOF

echo "========== config git proxy =========="
if [ "${http_proxy}" != "" ];then
    git config --global http.proxy ${http_proxy}
fi
if [ "${https_proxy}" != "" ];then
    git config --global https.proxy ${https_proxy}
fi
git config --list | grep proxy

echo "========== ping github.com =========="
ping -c 3 -W 10 github.com

echo "========== Clone hypercli repo =========="
mkdir -p /go/src/github.com/{hyperhq,docker}
cd /go/src/github.com/hyperhq
git clone https://github.com/hyperhq/hypercli.git

echo "========== Build hypercli =========="
cd /go/src/github.com/hyperhq/hypercli
if [[ "${PR_PRE}" == "#" ]];then
    echo "checkout pr :#${PR_NUMBER}"
    git fetch origin pull/${PR_NUMBER}/head:pr-${PR_NUMBER}
    git checkout pr-${PR_NUMBER}
else
    echo "checkout branch :${BRANCH}"
    git checkout ${BRANCH}
fi

if [[ $? -ne 0 ]];then
    echo "Branch ${BRANCH} not exist!"
    exit 1
fi

./build.sh
if [ $? -ne 0 ];then
    echo "Build hypercli failed"
    exit 1
fi
ln -s /go/src/github.com/hyperhq/hypercli /go/src/github.com/docker/docker
ln -s /go/src/github.com/hyperhq/hypercli/hyper/hyper /usr/bin/hyper
echo alias hypercli=\"hyper --region \${DOCKER_HOST}\" >> ~/.bashrc
source ~/.bashrc

echo "##############################################################################################"
echo "##                                 Welcome to integration test                              ##"
echo "##############################################################################################"
#show config for hyper cli
echo "Current hyper config: ${HYPER_CONFIG}/config.json"
echo "----------------------------------------------------------------------------------------------"
cat ${HYPER_CONFIG}/config.json \
  | sed 's/"secretkey":.*/"secretkey": "******************************",/g' \
  | sed 's/"auth":.*/"auth": "******************************"/g'
echo "----------------------------------------------------------------------------------------------"


## send begin message to slack
COMMIT_SHORT_ID=$(git rev-parse --short HEAD)
COMMIT_ID=$(git rev-parse HEAD)
COMMIT_URL="https://github.com/hyperhq/hypercli/commit/${COMMIT_ID}"

ATT_LINK="LINK: GITHUB(<${BRANCH_URL}|${BRANCH}> - <${COMMIT_URL}|${COMMIT_SHORT_ID}>) JOB(<${JOB_URL}|${BUILD_NUMBER}>)"
ATTACHMENT="[{'text':'$ATT_LINK'}]"
MESSAGE="[BEGIN] - $TITLE"
slack.sh "$MESSAGE" "$ATTACHMENT"

TEST_HOME="/go/src/github.com/hyperhq/hypercli/integration-cli"
cd /go/src/github.com/hyperhq/hypercli/integration-cli

cat <<EOF
##########################################
DOCKER_HOST:      ${DOCKER_HOST}
REGION:           ${REGION}
BRANCH:           ${BRANCH}
TEST_HOME:        ${TEST_HOME}
TEST_CASE_REG:    ${TEST_CASE_REG}
SLACK_TOKEN:      ${SLACK_TOKEN:0:15}-xxxxxxxxxxx
SLACK_CHANNEL_ID: ${SLACK_CHANNEL_ID}
##########################################
EOF

echo "========================================="
env
echo "========================================="

##############################################
# start test
##############################################
## first test
LOG="test1.log"
echo "====================first test(${TEST_CASE_REG} ${TIMEOUT} ${LOG})===================="
rm -rf $LOG >/dev/null 2>&1
script -ec "go test -check.f '${TEST_CASE_REG}' -timeout ${TIMEOUT:-90m}" | tee $LOG
ls -l $LOG
echo =========================

FAIL_COUNT1=`grep "^FAIL:" ${LOG} | wc -l`
TEST_RESULT1=`grep -E "^(OK:|OOPS:)" ${LOG}`
DURATION1=`grep -P "\tgithub.com/hyperhq/hypercli/integration-cli\t" ${LOG} | awk '{print $NF}'`

echo "----------get failed test case(1st)----------"
FAILED_FILE=failed1.log
cat ${LOG} | grep "^FAIL:" > ${FAILED_FILE}
FAILED_TEST_CASE1=`cat ${FAILED_FILE} | awk -F. '{if(NR==1){CASE=$NF}else{CASE=CASE"|"$NF}}END{printf CASE}'`
echo "-----------------------------------------"
echo "FAILED_TEST_CASE1: $FAILED_TEST_CASE1"


RETEST_CASE=""
RETEST_COUNT="0"
while read LINE
do
    if [ "${TARGET_REGION}" = "RegionOne" ]
    then
        HAS_VOL=`echo ${LINE} | grep -i Volume 2>/dev/null | wc -l`
        HAS_FIP=`echo ${LINE} | grep -i Fip 2>/dev/null | wc -l`
        if [ $HAS_VOL -ne 0 -o $HAS_FIP -ne 0 ]
        then
            echo "[SKIP FOR PKT]: ${LINE}"
            continue
        fi
    fi
    CASE_NAME=`echo ${LINE} | awk -F. '{printf $NF}'`
    if [ "${RETEST_CASE}" = "" ]
    then
        RETEST_CASE="$CASE_NAME"
    else
        RETEST_CASE="${RETEST_CASE}|$CASE_NAME"
    fi
    RETEST_COUNT=`expr $RETEST_COUNT + 1`
done < ${FAILED_FILE}

echo "-----------------------------------------"
echo "RETEST_CASE: ${RETEST_CASE}"
echo "-----------------------------------------"

if [ $RETEST_COUNT -ne 0 ];then
    ## second test
    LOG="test2.log"
    echo "====================second test(${RETEST_CASE} ${TIMEOUT} ${LOG})===================="
    rm -rf $LOG >/dev/null 2>&1
    script -ec "go test -check.f '${RETEST_CASE}' -timeout ${TIMEOUT:-90m}" | tee $LOG
    ls -l $LOG
    echo =========================

    echo "----------get failed test case(2nd)----------"
    FAILED_FILE=failed2.log
    cat ${LOG} | grep "^FAIL:" > ${FAILED_FILE}
    FAILED_TEST_CASE2=`cat ${FAILED_FILE} | awk -F. '{if(NR==1){CASE=$NF}else{CASE=CASE"|"$NF}}END{printf CASE}'`
    echo "-----------------------------------------"
    echo "FAILED_TEST_CASE2: $FAILED_TEST_CASE2"

    FAIL_COUNT2=`grep "^FAIL:" ${LOG} | wc -l`
    TEST_RESULT2=`grep -E "^(OK:|OOPS:)" ${LOG}`
    DURATION2=`grep -P "\tgithub.com/hyperhq/hypercli/integration-cli\t" ${LOG} | awk '{print $NF}'`

    END_TIME=`date "+%Y/%m/%d %H:%M:%S"`
    if [ $FAIL_COUNT2 -ne 0 ];then
        icon=":scream:"
        if [ "${TEST_RESULT1}" = "" -o "${TEST_RESULT2}" = "" ];then
            icon=":exclamation:"
        fi
        ATTACHMENT="[{'text':'${ATT_LINK}'},{'text':'DURATION(1st): ${DURATION1}'},{'text':'TEST_RESULT(1st): ${TEST_RESULT1}'},{'text':'RE_TEST_CASE(2nd): ${RETEST_CASE}'},{'text':'DURATION(2nd): ${DURATION2}'},{'text':'TEST_RESULT(2nd): ${TEST_RESULT2}'},{'text':'FAILED_TEST_CASE(2nd): $FAILED_TEST_CASE2'}]"
    else
        icon=":smile:"
        ATTACHMENT="[{'text':'${ATT_LINK}'},{'text':'DURATION(1st): ${DURATION1}'},{'text':'TEST_RESULT(1st): ${TEST_RESULT1}'},{'text':'RE_TEST_CASE(2nd): ${RETEST_CASE}'},{'text':'DURATION(2nd): ${DURATION2}'},{'text':'TEST_RESULT(2nd): ${TEST_RESULT2}'}]"
    fi
    echo "ATTACHMENT(1):${ATTACHMENT}"
    MESSAGE="[END] - ${TITLE} :${icon} - ${END_TIME}"
    else
    END_TIME=`date "+%Y/%m/%d %H:%M:%S"`
    ATTACHMENT="[{'text':'${ATT_LINK}'},{'text':'DURATION: ${DURATION1}'},{'text':'TEST_RESULT: ${TEST_RESULT1}'}]"
    icon=":smile:"
    if [ "${TEST_RESULT1}" = "" ];then
        icon=":exclamation:"
    fi
    if [ $FAIL_COUNT1 -ne 0 ];then
        echo "first failed, second passed"
        icon=":scream:"
        ATTACHMENT="[{'text':'${ATT_LINK}'},{'text':'DURATION: ${DURATION1}'},{'text':'TEST_RESULT: ${TEST_RESULT1}'},{'text':'FAILED_TEST_CASE: $FAILED_TEST_CASE1'}]"
    fi
    echo "ATTACHMENT(2):${ATTACHMENT}"
    MESSAGE="[END] - ${TITLE} :${icon} - ${END_TIME}"
fi

cat <<EOF
--------------------------------------
FAIL_COUNT1: ${FAIL_COUNT1}
DURATION1: ${DURATION1}
TEST_RESULT1: ${TEST_RESULT1}
FAILED_TEST_CASE1: ${FAILED_TEST_CASE1}

RETEST_COUNT: ${RETEST_COUNT}
RETEST_CASE: ${RETEST_CASE}

FAIL_COUNT2: ${FAIL_COUNT2}
DURATION2: ${DURATION2}
TEST_RESULT2: ${TEST_RESULT2}
FAILED_TEST_CASE2: ${FAILED_TEST_CASE2}
--------------------------------------
MESSAGE: ${MESSAGE}
ATTACHMENT: ${ATTACHMENT}
EOF

slack.sh "${MESSAGE}" "${ATTACHMENT}"
