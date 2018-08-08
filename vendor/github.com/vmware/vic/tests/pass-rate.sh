#!/bin/bash
# Copyright 2018 VMware, Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
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

# Depending on our end-of-year sprint schedule, the value of $(date +%W) may or may not need to be
# incremented by 1 prior to calculating the modulus in order for this to continue to align to our
# sprint calendar in each new calendar year.
sprint_start=$(gdate -d "last wednesday-$(( ($(gdate +%W 2>/dev/null)+0)%2 )) weeks" "+%Y-%m-%dT%H:%M:%S" 2>/dev/null) || sprint_start=$(date -d "last wednesday-$(( ($(date +%W)+0)%2 )) weeks" "+%Y-%m-%dT%H:%M:%S")

commits=$(curl -s https://api.github.com/repos/vmware/vic/commits?access_token=$GITHUB_AUTOMATION_API_KEY\&since=$sprint_start | jq -r ' map(.sha) | join(",")')
curl -s https://api.github.com/repos/vmware/vic/status/{$commits}?access_token=$GITHUB_AUTOMATION_API_KEY | jq '.statuses[] | select(.context == "continuous-integration/vic/push") | "\(.target_url): \(.state)"' | tee status.out

failures=$(cat status.out | grep -c failure)
successes=$(cat status.out | grep -c success)
pending=$(cat status.out | grep -c pending)

let complete=$successes+$failures
if [ $complete -eq 0 ]; then
    # This should be "undefined", but starting at 100% seems reasonable given how this is used.
    passrate="100.00"
else
    passrate=$(bc -l <<< "scale=2;100 * ($successes / $complete)")
fi

echo "Number of failed runs on merges to master in the $complete completed builds since $sprint_start: $failures"
echo "Number of successful runs on merges to master in the $complete completed builds since $sprint_start: $successes"
echo "Number of runs since $sprint_start which are still pending: $pending"

echo "Current vmware/vic CI passrate: $passrate (of completed)"
if [ $complete -eq 0 ]; then
    curl --max-time 10 --retry 3 -s --data-urlencode "payload={'channel': '#notifications', 'text': 'Current <https://github.com/vmware/vic|vmware/vic> CI passrate: $passrate% (of $complete completed this sprint)'}" "$SLACK_URL"
else
    curl --max-time 10 --retry 3 -s --data-urlencode "payload={'channel': '#notifications', 'attachments':[{'text': 'Current <https://github.com/vmware/vic|vmware/vic> CI passrate: $passrate% (of $complete completed this sprint)', 'image_url': 'http://chart.googleapis.com/chart?cht=p&chs=250x150&chco=dc3912|109618|3366cc&chl=failure|success|pending&chd=t:$failures,$successes,$pending', 'footer':'pass-rate.sh'}]}" "$SLACK_URL"
fi
