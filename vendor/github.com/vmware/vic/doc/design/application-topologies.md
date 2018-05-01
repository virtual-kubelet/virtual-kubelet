# Application topologies

This describes the core application topologies that will be used to validate function and determine relative priority of feature implementation. This is by no means a complete list and the ordering may change over time as we gain additional feedback from users.

## Prime topology - platform 2.5 tiered application

### Description
This is the topology currently being used as the primary touchstone for feature support and priority. It's based off [the docker voting app](https://github.com/docker/example-voting-app) as a great example of a mapping from a traditional tiered application to a containerized environment. This is not assumed to be a pure [twelve factor app](http://12factor.net/) because while that significantly simplifies the infrastructure requirements it's not a viable assumption for all workloads.

There are two scenarios described in the breakdown:
1. unmodified voting app
2. voting app using non-containerized database with direct exposure to an external network on the front-end

[Application workflow](https://github.com/vmware/vic/doc/application_workflow) has examples on building these applications.  The [voting app](https://github.com/vmware/vic/doc/application_workflow/voting_app.md) illustrates scenario 1, discussed above.  In this example, the entire voting app and all tiers are created as a self-enclosed set of containers.  Docker-compose is used to orchestrate the deployment.

The second scenario is assumed to be a better reflection of probable deployment scenarios as people are moving to containerized workloads, hence the description of this as a 2.5 platform application.

TODO: Add an example to the application workflow docs illustrating scenario 2.
