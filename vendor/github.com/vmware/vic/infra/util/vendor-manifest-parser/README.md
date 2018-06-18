### Dependency parsing tool

This tool is here to simplify checking the vendor manifest for consistency and for which tags are being used

It queries GitHub and takes into account any specific exceptions and produces a human-readable report on what our dependencies are

#### Building

There's no point building this tool all the time. It will be used sporadically for auditing purposes.
Simply use go build <sourcefile> to create the binaries when needed

#### Running

The binaries are designed to be piped together to take each others output. This way, any intermediate json can be fed into a spreadsheet or likewise

Example usage:

```
cat ../../manifest | gettags --uid "bcorrie@vmware.com" --pwd "foobedoo" > 1.json
cat 1.json | groupbyrepo > 2.json
cat 2.json | report --exceptionFile=../../exceptions

cat ../../manifest | gettags --uid "bcorrie@vmware.com" --pwd "foobedoo" | groupbyrepo | report --exceptionFile=../../exceptions
```

#### Exceptions

It would be good practice for us to document cases in which a specific revision must be used, either because of a critical patch or equivalent. See exceptions file co-located with the manifest for examples.

Exceptions are outputted in the report and ensure that no-one attempts to modify a revision to a later tag without considering the exception