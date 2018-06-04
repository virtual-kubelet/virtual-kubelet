# Container Workflow

The following are guideline suggestions for those who want to use VIC to develop and deploy a containerized application.  These guidelines pertain to VIC 0.7.0.  While VIC continues to progress towards VIC 1.0, the current feature set requires some care in creating a containerized application and deploying it.  We present these guidelines from a developer and a devops perspective.

An example workflow is presented here, in the form of a modified voting app, based on [Docker's voting app](https://github.com/docker/example-voting-app).

## Container Workflow Feature Set

General feature set (e.g. Docker run, ps, inspect, etc) used at the CLI will not be discussed.  Only feature set important for containerizing apps and deploying them are discussed.  These include volume and network support.  It is also worth mentioning that basic Docker Compose support is available for application deployment.

#### Currently Available Features

1. Docker Compose (basic)
2. Registry pull from docker hub and private registry
3. Named Data Volumes
4. Anonymous Data Volumes
5. Bridged Networks
6. External Networks
7. Port Mapping
8. Network Links/Alias

#### Future Features

Be aware the following feature are not yet available and must be taken into account when containerizing an app and deploying it.

1. Docker build
2. Registry pushing
3. Concurrent data volume sharing between containers
4. Local host folder mapping to a container volume
5. Local host file mapping to a container
6. Docker copy files into a container, both running and stopped
7. Docker container inspect does not return all container network for a container

## Workflow Guidelines

Anything that can be performed with Docker Compose can be performed manually via the Docker CLI and via scripting using the CLI.  This makes Compose a good baseline reference and our guidelines will use it for demonstration purposes.  Our guideline uses Docker Compose 1.8.1.  The list above in the Future Features section puts constraints on what types of containerized application can be deployed on VIC 0.7.0.

Please note, these guidelines and recommendations exist for the current feature set in VIC 0.7.0.  As VIC approaches 1.0, many of these constraints will go away.

#### Guidelines for Building Container Images
The current lack of docker build and registry pushing means users will need to use regular Docker to build a container and to push it to the global hub or your corporate private registry.  The example workflow using Docker's voting app will illustrate how to get around this constraint.

#### Guidelines for Sharing Config
VIC 0.7.0 current lack of data volume sharing and docker copy will put constraints on how configuration are provided to a containerized application.  An example of configuration is your web server config files.  Our recommendation for getting around the current limitation is to pass in configuration via command line arguments or environment variables.  Add a script to the container image that ingest the command line argument/environment variable and pass these configuration to the contained application.  A benefit of using environment variables to transfer configuration is the containerized app will more closely follow the popular 12-factor app model.

With no direct support for sharing volumes between containers processes that must share files have the following options:

1. build them into the same image and run in the same container
2. add a script to the container that mounts an NFS share (containers must be on the same network)
   a. Run container with NFS server sharing a data volume
   b. Mount NFS share in whichever containers need to share

TODO: Provide example of both

## Example Applications

We have taken Docker's voting app example and used the above guidelines to modify it for use on VIC 0.7.0.  Please follow to this [page](voting_app.md) for more information.
