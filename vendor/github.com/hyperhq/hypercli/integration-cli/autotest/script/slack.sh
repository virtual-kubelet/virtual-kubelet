#!/usr/bin/env bash

if [ "${SLACK_TOKEN}" == "" -o "${SLACK_CHANNEL_ID}" == "" ];then
    echo "SLACK_TOKEN or SLACK_CHANNEL_ID is unset, skip send slack message"
    exit 0
fi

NICK="HykinsBot"
EMOJI=":jenkins:"

MESSAGE="$1"
ATTACHMENT="$2"

curl -X POST \
-F token=${SLACK_TOKEN} \
-F channel=${SLACK_CHANNEL_ID} \
-F "text=${MESSAGE}" \
-F username=$NICK \
-F "attachments=${ATTACHMENT}" \
-F "icon_emoji=${EMOJI}" \
https://slack.com/api/chat.postMessage

echo
if [ $? -eq 0 ];then
    echo "send slack message OK :)"
else
    echo "send slack message FAILED :("
fi