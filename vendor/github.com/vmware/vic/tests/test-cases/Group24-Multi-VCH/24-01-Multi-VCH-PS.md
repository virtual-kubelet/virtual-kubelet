# Test 24-01 - Create Multi VCH - Docker Ps Only Contains The Correct Containers

## Purpose
Verify that one VCH's `docker ps` output does not contain the containers of another VCH.

# Environment: 
This test requires that a vsphere server is running and available. Additionally, it will require the
ability to install two vch's to the target VC/ESXi.

## Test Steps:
1. Install two VCH's
2. Create a container on VCH 1
3. Create a container on VCH 2
4. Run `docker ps -a` on VCH 1
5. Run `docker ps -a` on VCH 2

## Expected Outcome
1. Both VCH's should install successfully and without issue.
2. VCH 1 should successfully create a container.
3. vch 2 should successfully create a container.
4. VCH 1 `docker ps -a` output should only contain the container it made.
5. VCH 2 `docker ps -a` output should only container the container it made.
