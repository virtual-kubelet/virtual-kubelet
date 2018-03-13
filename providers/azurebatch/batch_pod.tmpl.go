package azurebatch

// Todo: Investigate a better way to inline this template - especially when escaping the backticks.
// Consider: https://mattjibson.com/blog/2014/11/19/esc-embedding-static-assets/
const azureBatchPodTemplate = `
{{/* Vars */}}
{{$podName := .PodName}}
{{$volumes := .Volumes}}

{{/* Login to required image repositories */}}
{{range .PullCredentials }}
docker login -u {{.Username}} -p {{.Password}} {{.Server}}
{{end}}

{{/* Create Pod network and start it */}}
docker network create {{.TaskID}}
docker run -d --network {{.TaskID}} --cidfile="./container-net.cid" gcr.io/google_containers/pause:1.0  
networkContainerID=$(<./container-net.cid)

{{/* Handle volumes */}}
{{range .Volumes}}
{{if isHostPathVolume .}}
docker volume create --name {{$podName}}_{{.Name}} --opt type=none --opt device={{.VolumeSource.HostPath.Path}} --opt o=bind
{{end}}
{{if isEmptyDirVolume .}}
docker volume create {{$podName}}_{{.Name}}
{{end}}
{{end}}

{{/* Run the containers in the Pod. Attaching to shared namespace */}}
{{range $index, $container := .Containers}}
docker run -d --network container:$networkContainerID --ipc container:$networkContainerID \
    {{- range $index, $envs := $container.Env}}
-e "{{$envs.Name}}:{{$envs.Value}}" \
    {{- end}}
    {{- range $index, $mount := getValidVolumeMounts $container $volumes}}
-v {{$podName}}{{$mount.Name}}:{{$mount.MountPath}} \
    {{- end}}
--cidfile=./container-{{$index}}.cid {{$container.Image}} {{getLaunchCommand $container}}
{{end}}

{{/* Symlink all container logs files to task directory */}}
{{/* sudo ls item-* | xargs -n 1 cat | sudo xargs -n 1 docker inspect --format='{{.LogPath}}' | xargs -n 1 -i ln -s {} ./barry.txt */}}
{{range $index, $container := .Containers}}
container_{{$index}}_ID=$(<./container-{{$index}}.cid)
container_{{$index}}_Log_Path=$(docker inspect --format='{{"{{.LogPath}}"}}' $container_{{$index}}_ID)
ln -s $container_{{$index}}_Log_Path ./{{$container.Name}}.log
{{end}}

echo Date
echo "Running Pod: {{.PodName}}"

{{/* Wait until any of these containers stop */}}
echo "Waiting for any of the containers to exit"
for line in ` + "`ls container-*`" + `
do    
id=$(cat $line) 
docker wait $id &
done

wait -n

{{/* Get exit codes from containers */}}
echo "Checking container exit codes"
overallExitCode=0
for line in ` + "`ls container-*`" + `
do    
    id=$(cat $line) 
    exitCode=$(docker inspect $id | jq '.[].State.ExitCode')
    echo "ID:$id ExitCode: $exitCode"
    if [ $exitCode -gt 0 ]
    then
        $overallExitCode=$exitCode
    fi
done



{{/* Remove the containers, network and volumes */}}
echo "A container exited: Removing all containers"

for line in ` + "`ls container-*`" + `
do    
id=$(cat $line) 
echo "-Printing log"
docker logs $id
echo "-Removing"
docker rm -f $id
done

docker network rm {{.TaskID}}

{{range .Volumes}}
docker volume rm {{$podName}}_{{.Name}}
{{end}}

exit $overallExitCode
`
