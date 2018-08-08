# Test 24-02 - VCH delete only removes its own containerVMs

## Purpose
Verify that in an environment with multiple VCHs installed, vic-machine delete removes the targeted VCH and only its containerVMs (not those of other VCHs).

## Environment
This test requires a running and available vSphere server.

## Test Steps
1. Install a VCH
2. Create a busybox container
3. Install another VCH
4. Create a busybox container on the VCH from Step 3
5. Clean up the VCH from Step 3
6. Use govc to look for the first VCH's containerVM
7. Clean up the VCH from Step 1

## Expected Outcome
* All steps should succeed
* Step 6's output should contain the name of the first VCH's container
