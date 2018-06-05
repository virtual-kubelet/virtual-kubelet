### Docker API Filtering

The [docker api server](components.md#docker-api-server) is the endpoint for the docker client.  In many of the docker
client commands a user is able to specify filter criteria.  That criteria will be evaluated in
the docker persona and will limit the results of the docker command.

#### Docker Filters

The docker client provides filter options across a variety of commands.  A good example of
docker filter options is the `docker images` command.  The following options are available
as of Docker 1.13:

```
-f, --filter value    Filter output based on conditions provided (default [])
                        - dangling=true
                        - label=<key> or label=<key>=<value>
                        - before=(<image-name>[:tag]|<image-id>|<image@digest>)
                        - since=(<image-name>[:tag]|<image-id>|<image@digest>)
                        - reference=(<image-name>[:tag])
```

Here's an example using the images filter: `docker images -f label=prod`.  That command will
return only the images with a label key of `prod`.

#### Filter Design

The vic filtering logic is provided in the filter package of the vic docker personality.  That
package should hold all of the vic implemented logic for docker personality filtering. The
filtering logic is a combination of new vic code and the reuse of docker validation code.

The package contains the common filtering and validation logic that can be used across the
vic docker personality.  Additionally the package contains object specific filtering logic
isolated in object specific files.  For example all vic specific container filtering will
be provided in the following file: `/lib/apiservers/engine/backends/filter/container.go`.
The vic container filtering will take advantage of the common functionality as well as
implement vic specific container filtering logic.

##### Common Filtering

There are a few filter options that span multiple docker client commands.  That filtering
logic is contained in the `/lib/apiservers/engine/backends/filter/filter.go` file.  The
following common filtering is provided:
	- ID
	- Name
	- Label
	- Before
	- Since
If a docker command provides any of these filtering options then the vic implementation
should take advantage of this common filtering mechanism.

##### Object Specific Filtering

As mentioned above object specific filtering should be isolated in a file identifying the
object.  For example all vic specific container filtering will be provided in the
following file: `/lib/apiservers/engine/backends/filter/container.go`.

Object specific filtering should take advantage of the aforementioned common filtering
as well as vendored docker validation code.  Then any additional custom vic filtering
should be provided in the file.

##### Docker filter support

While all docker filtering options are valid not all filtering options are supported
by vic.  In order to segregate validity from support each command that provides filtering
must provide maps indicating valid vs. supported.  If all docker filtering options are
supported by vic then only one map will be needed.  The container object and specifically
the `docker ps` command provides a good example of the [maps](https://github.com/vmware/vic/blob/master/lib/apiservers/engine/backends/container.go#L60).

There are two maps for the container object since not all of the filter criteria is
currently supported by vic.  Those two maps are provided to the common filter validation
code to check for validity and supportability.

If a command is not currently supported by vic the validation logic will provide the
following error message:
```
	Error response from daemon: filter dangling is not currently supported by vic
```

