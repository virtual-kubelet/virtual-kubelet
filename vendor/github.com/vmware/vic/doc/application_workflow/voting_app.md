# Voting App on VIC

The [voting app](https://github.com/docker/example-voting-app) is one of Docker's example multi-tiered app.  We will neither discuss the design of the app nor explain how it works.  Instead, we will focus on it's Docker Compose yml file and use the [guidelines](README.md) to modify that yml file to make it work on VIC 0.7.0.  You can find the modified compose yml file in the folder [vic/demos/compose/voting-app/](../../demos/compose/voting-app).  We have only included the modified yml file as the below workflow will use the original source from github.

## Workflow

### Original Compose File

```
version: "2"

services:
  vote:
    build: ./vote
    command: python app.py
    volumes:
     - ./vote:/app
    ports:
      - "5000:80"

  redis:
    image: redis:alpine
    ports: ["6379"]

  worker:
    build: ./worker

  db:
    image: postgres:9.4

  result:
    build: ./result
    command: nodemon --debug server.js
    volumes:
      - ./result:/app
    ports:
      - "5001:80"
      - "5858:5858"
```

We see the above compose file are using two features that are not yet supported in VIC 0.7.0.  The first is docker build.  The second is local folder mapping to container volume.  Let's walk through modifying this app and deploy it onto a vSphere environment.

### Getting the App Prepared

First, clone the repository from github.  Note, we have included the modified compose file in our /demos folder, but in this exercise, we are going to modify this app from the sources from github.

Second, to get around the docker build directive, we follow the previously mentioned guidelines and use regular docker to build each component that requires a build.  Then we tag the the images to upload to our private registry (or private account on Docker Hub).  In this example, we are going to use VMWare's victest account on Docker Hub.  You will not be able to use this account, but you can create your own and use that in place of the victest keywords below.  Please note, the steps shown below are performed in a terminal using regular docker (as opposed to VIC's docker personality daemon).  Note, it is possible to build and tag an image in one step.  Below, the steps are broken into separate steps.

**build the images:**  
$> cd example-voting-app  
$> docker build -t vote ./vote  
$> docker build -t vote-worker ./worker  
$> docker build -t vote-result ./result  

**tag the images for a registry:**  
$> docker tag vote victest/vote  
$> docker tag vote-worker victest/vote-worker  
$> docker tag vote-result victest/vote-result  

**push the images to the registry:**  
$> docker login (... and provide credentials)  
$> docker push victest/vote  
$> docker push victest/vote-worker  
$> docker push victest/vote-result  

Second, we analyze the application.  There doesn't appear to be a real need to map the local folder to the container volume so we remove the local folder mapping.  We also remove all the build directives from the yml file.

### Updated Compose File for VIC 0.7.0

```
version: "2"

services:
  vote:
    image: victest/vote
    command: python app.py
    ports:
      - "5000:80"

  redis:
    image: redis:alpine
    ports: ["6379"]

  worker:
    image: victest/vote-worker

  db:
    image: postgres:9.4

  result:
    image: victest/vote-result
    command: nodemon --debug server.js
    ports:
      - "5001:80"
      - "5858:5858"
```

### Deploy to Your VCH

We assume a VCH has already been deployed with vic-machine and VCH_IP is the IP address of the deployed VCH.  This IP should have been presented after the VCH was successfully installed.  We also assume we are still in the example-voting-app folder, with the modified compose yml file.

$> docker-compose -H VCH_IP up -d

Now, use your web browser to navigate to "http://VCH_IP:5000" and "http://VCH_IP:5001" to verify the voting app is running.

That's really all there is to deploying **this** app.  It is a contrived app and more complex containerized apps may have more steps to perform before it will on VIC 0.7.0.
