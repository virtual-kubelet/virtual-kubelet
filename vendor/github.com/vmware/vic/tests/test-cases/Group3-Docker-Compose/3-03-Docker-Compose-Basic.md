Test 3-03 - Docker Compose Basic
=======

# Purpose:
To verify that VIC appliance can work when deploying the most basic example from docker documentation

# References:
[1 - Docker Compose Getting Started](https://docs.docker.com/compose/gettingstarted/)

# Environment:
This test requires that a vSphere server is running and available

# Test Steps:
1. Create a compose file that includes a basic python server and redis server
2. Deploy VIC appliance to the vSphere server
3. Create a basic compose yml file
4. Issue DOCKER_HOST=<VCH IP> docker-compose create
5. Issue DOCKER_HOST=<VCH IP> docker-compose start
6. Issue DOCKER_HOST=<VCH IP> docker-compose logs
7. Issue DOCKER_HOST=<VCH IP> docker-compose stop
8. Issue DOCKER_HOST=<VCH IP> docker-compose up -d
9. Issue DOCKER_HOST=<VCH IP> docker-compose kill redis
10. Issue DOCKER_HOST=<VCH IP> docker-compose down
11. Issue DOCKER_HOST=<VCH IP> docker run -d busybox /bin/top
12. Issue DOCKER_HOST=<VCH IP> docker-compose up -d
13. Create compose file with link
14. Issue DOCKER_HOST=<VCH IP> docker-compose up -d
15. Issue DOCKER_HOST=<VCH IP> docker-compose bundle
16. Issue DOCKER_HOST=<VCH IP> docker-compose --file compose-rename.yml up -d
17. Issue DOCKER_HOST=<VCH IP> docker-compose --file compose-rename.yml up -d --force-recreate
18. Issue DOCKER_HOST=<VCH IP> docker-compose --file compose-rename.yml up -d
19. Issue DOCKER_HOST=<VCH IP> docker-compose up (without -d)
20. Issue DOCKER_HOST=<VCH IP> docker-compose stop on the above container
21. Issue DOCKER_HOST=<VCH IP> docker-compose up (without -d) again

# Expected Outcome:
* Step 4-21 should result in no errors

# Possible Problems:
None