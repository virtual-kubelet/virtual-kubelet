package pod2docker

// Todo: Investigate a better way to inline this template - especially when escaping the backticks.
// Consider: https://mattjibson.com/blog/2014/11/19/esc-embedding-static-assets/
const azureBatchPodTemplate = `
#!/bin/bash
set -eE 
trap cleanup EXIT

if ! type 'docker' > /dev/null; then
  echo 'Docker not installed... exiting'
  exit 1
fi

{{/* Vars */}}
{{$podName := .PodName}}
{{$volumes := .Volumes}}

{{/* Login to required image repositories */}}
{{range .PullCredentials }}
docker login -u {{.Username}} -p {{.Password}} {{.Server}}
{{end}}

function cleanup(){
    {{/* Take a copy of the container log is removed when container is deleted */}}
    echo 'Pod Exited: Copying logs' 
    {{range $index, $container := .InitContainers}}
    if [[ -f ./initcontainer-{{$index}}.cid ]]; then
        container_{{$index}}_ID=$(<./initcontainer-{{$index}}.cid)
        container_{{$index}}_Log_Path=$(docker inspect --format='{{"{{.LogPath}}"}}' $container_{{$index}}_ID)
        cp $container_{{$index}}_Log_Path ./{{$container.Name}}.log

        docker rm -f $container_{{$index}}_ID
        rm -f ./initcontainer-{{$index}}.cid
    fi
    {{end}}

    {{range $index, $container := .Containers}}
    if [[ -f ./{{$container.Name}}.log && -f ./container-{{$index}}.cid ]]; then
        container_{{$index}}_ID=$(<./container-{{$index}}.cid)
        container_{{$index}}_Log_Path=$(docker inspect --format='{{"{{.LogPath}}"}}' $container_{{$index}}_ID)
        rm ./{{$container.Name}}.log {{/* Remove the existing symlink */}}
        cp $container_{{$index}}_Log_Path ./{{$container.Name}}.log
    fi
    {{end}}

    {{/* Remove the containers, network and volumes */}}

    echo 'Pod Exited: Removing all containers'
    if ls container-* 1> /dev/null 2>&1; then
        for line in ` + "`ls container-*`" + `
        do    
            id=$(cat $line)
            echo '-Logs container..'
            docker logs $id
            echo '-Removing container..'
            docker rm -f $id
            rm -f $line
        done    
    fi
    echo '-Removing pause container..'
    docker rm -f {{$podName}} || echo 'Remove pause container failed'
    rm -f ./pauseid.cid
    echo '-Removing network container..'
    docker network rm {{$podName}} || echo 'Remove network failed'
    
    echo '-Removing volumes..'        
    {{range .Volumes}}
    docker volume rm -f {{$podName}}_{{.Name}} || echo 'Remove volume failed'
    {{end}}
}

{{/* Create Pod network and start it */}}
docker network create {{$podName}}
docker run -d --network {{$podName}} --name {{$podName}} --cidfile="./pauseid.cid" gcr.io/google_containers/pause:1.0  

{{/* Handle volumes */}}
{{range .Volumes}}
{{if isHostPathVolume .}}
docker volume create --name {{$podName}}_{{.Name}} --opt type=none --opt device={{.VolumeSource.HostPath.Path}} --opt o=bind
{{end}}
{{if isEmptyDirVolume .}}
docker volume create {{$podName}}_{{.Name}}
{{end}}
{{end}}

{{/* Run the init containers in the Pod. Attaching to shared namespace */}}
{{range $index, $container := .InitContainers}} 
echo 'Running init container {{$index}}..'
    {{if isPullAlways .}}
docker pull {{$container.Image}}
    {{end}}
docker run --network container:{{$podName}} --ipc container:{{$podName}} \
    {{- if isNvidiaRuntime $container}}
    --runtime nvidia \
    {{- end}}
    {{- range $index, $envs := $container.Env}}
-e "{{$envs.Name}}:{{$envs.Value}}" \
    {{- end}}
    {{- range $index, $mount := getValidVolumeMounts $container $volumes}}
-v {{$podName}}_{{$mount.Name}}:{{$mount.MountPath}} \
    {{- end}}
--cidfile=./initcontainer-{{$index}}.cid {{$container.Image}} {{getLaunchCommand $container}}
{{end}}


{{/* Run the containers in the Pod. Attaching to shared namespace */}}
{{range $index, $container := .Containers}} 
    {{if isPullAlways .}}
docker pull {{$container.Image}}
    {{end}}
docker run -d --network container:{{$podName}} --ipc container:{{$podName}} \
    {{- if isNvidiaRuntime $container}}
    --runtime nvidia \
    {{- end}}
    {{- range $index, $envs := $container.Env}}
-e "{{$envs.Name}}:{{$envs.Value}}" \
    {{- end}}
    {{- range $index, $mount := getValidVolumeMounts $container $volumes}}
-v {{$podName}}_{{$mount.Name}}:{{$mount.MountPath}} \
    {{- end}}
--cidfile=./container-{{$index}}.cid {{$container.Image}} {{getLaunchCommand $container}}
{{end}}

{{/* Symlink all container logs files to task directory */}}
{{range $index, $container := .Containers}}
container_{{$index}}_ID=$(<./container-{{$index}}.cid)
container_{{$index}}_Log_Path=$(docker inspect --format='{{"{{.LogPath}}"}}' $container_{{$index}}_ID)
ln -f -s $container_{{$index}}_Log_Path ./{{$container.Name}}.log
{{end}}

echo 'Running Pod: {{.PodName}}'

{{/* Wait until any of these containers stop */}}
echo 'Waiting for any of the containers to exit'
for line in ` + "`ls container-*`" + `
do    
    id=$(cat $line) 
    docker wait $id &
done

while [ $(jobs -p | wc -l) == {{.Containers | len}} ]
do
   sleep 2
done


{{/* Get exit codes from containers */}}
echo 'Checking container exit codes'
overallExitCode=0
for line in ` + "`ls container-*`" + `
do    
    id=$(cat $line) 
    echo 'Getting exitcode'
    exitCode=$(docker inspect -f {{"{{.State.ExitCode}}"}} $id)
    
    echo 'ID: ' $id ' ExitCode: ' $exitCode
    echo 'Checking exitcode'
    if (($exitCode != 0))
    then
        echo 'Assigning exitcode'
        overallExitCode=$exitCode
    fi
done

exit $overallExitCode
`
