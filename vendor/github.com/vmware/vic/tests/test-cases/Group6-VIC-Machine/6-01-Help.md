Test 6-01 - Verify Help
=======

# Purpose:
Verify vic-machine delete help

# References:
* vic-machine-linux delete -h

# Environment:
Standalone test requires nothing but vic-machine to be built

# Test Cases

## Inspect help basic
1. Issue the `vic-machine-linux inspect -h` command

## Delete help basic
1. Issue the `vic-machine-linux delete -h` command

### Expected Outcome:
* Command should output the usage of vic-machine inspect -h:
```
vic-machine-linux inspect - Inspect VCH
```
* Command should output the usage of vic-machine delete -h:
```
vic-machine-linux delete - Delete VCH
```
