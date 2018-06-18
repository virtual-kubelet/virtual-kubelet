Test 6-08 - Verify vic-machine create compute-resource verification
=======

# Purpose:
Verify vic-machine create compute resource parsing

# References:
* vic-machine-linux create -h

# Environment:
This test requires that a vSphere server is running and available

# Wrong absolute path
# Test Steps
1. Create with compute resource set wrongDC:malformat
2. Create with compute resource set to /WrongDC/cluster/
3. Create with compute resource set to /DC/cluster/host/rp
4. Verify creation failed correctly

# Correct absolute path
# Test Steps
1. Create with compute resource set to /DC/host/cluster/Resources/rp
2. Verify creation passed successfully
3. Verify VCH is created in the correct place though govc

# Wrong relative path
# Test Steps
1. Prepare env with multiple VC clusters and multiple available resource pools
2. Create with compute resource set to wrongRP1 (wrongRP1 does not exist)
3. Create with compute resource set to wrongCluster (wrongCluster does not exist)
4. Create with compute resource set to RP1 (RP1 exists in one cluster)
5. Create with compute resource set to Cluster1 (Cluster1 exists)
6. Verify creation failed correctly

# Correct relative path with single VC cluster
# Test Steps
1. Prepare env single VC cluster
2. Create with compute resource set to <cluster name> (real cluster name here)
3. Create with compute resource set to RP1 (RP1 exists in cluster)
4. Create with compute resource not set
5. Verify deployed successfully
6. Verify VCH is created in the correct place though govc

# Correct relative path with ESXi
# Test Steps
1. Test in ESXi
2. Create with compute resource set to RP1 (RP1 exists in cluster)
3. Create with compute resource not set
4. Verify deployed successfully
5. Verify VCH is created in the correct place though govc

# Correct relative path in multiple VC cluster
# Test Steps
1. Prepare env with multiple VC clusters and multiple available resource pools
2. Create with compute resource set to Cluster (Cluster exists)
3. Create with compute resource set to Cluster/RP1 (Cluster/RP1 exists in cluster)
4. Verify deployed successfully
5. Verify VCH is created in the correct place though govc
