package azure

// Todo: Investigate a better way to inline this template - especially when escaping the backticks.
// Consider: https://mattjibson.com/blog/2014/11/19/esc-embedding-static-assets/
const azureBatchPodTemplate = `
{{/* Login to required image repositories */}}
{{range $index, $pullCreds := .PullCredentials }}
docker login -u {{$pullCreds.Username}} -p {{$pullCreds.Password}} {{$pullCreds.Server}}
{{end}}

{{/* Create Pod network and start it */}}
docker network create {{.TaskID}}
docker run -d --network {{.TaskID}} --cidfile="./container-net.cid" gcr.io/google_containers/pause:1.0 
networkContainerID=$(<./container-net.cid)

{{/* Todo: Handle volumes */}}

{{/* Run the containers in the Pod. Attaching to shared namespace */}}
{{range $index, $container := .Containers}}
docker run -d --network container:$networkContainerID --ipc container:$networkContainerID \
    {{- range $index, $envs := $container.Env}}
-e "{{$envs.Name}}:{{$envs.Value}}" \
    {{- end}}
--cidfile=./container-{{$index}}.cid {{$container.Image}} {{getLaunchCommand $container}}
{{end}}

{{/* Symlink all container logs files to task directory */}}
{{/* sudo ls item-* | xargs -n 1 cat | sudo xargs -n 1 docker inspect --format='{{.LogPath}}' | xargs -n 1 -i ln -s {} ./barry.txt */}}
{{range $index, $container := .Containers}}
container_{{$index}}_ID=$(<./container-{{$index}}.cid)
container_{{$index}}_Log_Path=$(docker inspect --format={{` + "`'{{.LogPath}}'`" + `}} $container_{{$index}}_ID)
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


{{/* Remove the containers */}}
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
`
