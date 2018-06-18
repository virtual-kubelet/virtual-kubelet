Test 6-14 - Verify vic-machine update firewall function
=======

# Purpose:
Verify vic-machine update firewall

# References:
* vic-machine-linux update firewall -h

# Environment:
This test requires that a vSphere server is running and available



Update
=======

## Enable and disable VIC firewall rule
1. Get state of host firewall
2. Enable host firewall
3. Verify host firewall enabled
4. Enable VIC firewall rule by issuing the following command:
```
bin/vic-machine-linux update firewall --target %{TEST_URL} \
    --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} \
    --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} \
    --allow
```

5. Verify state of rule by issuing the following command:
```
govc host.esxcli network firewall ruleset list --ruleset-id=vSPC
```

6. Create VCH
7. Run regression tests
8. Disable VIC firewall rule by issuing the following command:
```
bin/vic-machine-linux update firewall --target %{TEST_URL} \
    --user=%{TEST_USERNAME} --password=%{TEST_PASSWORD} \
    --compute-resource=%{TEST_RESOURCE} --timeout %{TEST_TIMEOUT} \
    --deny
```

9. Verify state of rule by issuing the following command:
```
govc host.esxcli network firewall ruleset list --ruleset-id=vSPC
```

10. Revert state of host firewall

### Expected Outcome
* Firewall rule state changes as expected
* Regression tests pass
