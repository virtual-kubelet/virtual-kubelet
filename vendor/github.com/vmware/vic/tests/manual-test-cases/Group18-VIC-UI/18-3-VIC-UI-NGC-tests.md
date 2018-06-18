Test 18-3 - VIC UI NGC Tests
======

# Purpose:
To test user interactions with VIC UI in vSphere Web Client

# References:

# Environment:
* Testing VIC UI requires a working VCSA setup with VCH installed

# Test Steps:
1. Check if provider properties files exist
2. Ensure UI plugin is already registered with VC before testing
3. Run the NGC tests
  - Test 1: Verify if the VIC UI plugin is installed correctly
    - Open a browser
    - Log in as admin user
    - Navigate to Administration -> Client Plug-Ins
    - Verify if item “VicUI” exists

  - Test 2.1: Verify if VCH VM Portlet exists
    - Open a browser
    - Log in as admin user
    - Navigate to the VCH VM Summary tab
    - Verify if property id `dockerApiEndpoint` exists

  - Test 2.2: Verify if VCH VM Portlet displays correct information while VM is OFF
    - Ensure the vApp is off
    - Open a browser
    - Log in as admin user
    - Navigate to the VCH VM Summary tab
    - Verify if `dockerApiEndpoint` equals the placeholder value `-`

  - Test 2.3: Verify if VCH VM Portlet displays correct information while VM is ON
    - Ensure the vApp is on
    - Open a browser
    - Log in as admin user
    - Navigate to the VCH VM Summary tab
    - Verify if `dockerApiEndpoint` does not equal the placeholder value `-`

  - Test 3: Verify if Container VM Portlet exists
    - Open a browser
    - Log in as admin user
    - Navigate to the Container VM Summary tab
    - Verify if property id `containerName` exists

# Expected Outcome:
* Each step should return success

# Possible Problems:
1. NGC automated testing is not available on VC 5.5, so if the tests were to run against a box with VC 5.5 Step 3 above would be skipped. However, you can manually run the NGC tests by following the steps above.
2. Some Selenium Web Drivers are known to have bugs that slow down or even crash the tests
  - 64 bit version of the Internet Explorer Driver has an issue with text input speed where it takes about 4-5 seconds per keystroke. (using the 32 bit version solves the issue)
  - When run with the Chrome Driver, tests fail at the login page of the vSphere Web Client; browser hangs and does not automatically enter username and password
  - Firefox driver has been the most stable and thus was set as the default browser for testing
  - The findings were made using the latest non-beta release of Selenium (v2.53.1) and the latest browser drivers on a Nimbus-based Windows 7 VM
