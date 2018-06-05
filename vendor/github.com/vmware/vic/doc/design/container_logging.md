# Container VM output logging in VIC

When a container VM process produces output, that output is sent down through a serial connection into the vSphere backend where it is written to a logfile. When a user wants to see this output, they use the `docker logs` command, which reads from this logfile and displays the log entries on the command line in order from oldest to newest.

However, in order for users to be able to use the `--since` option with `docker logs`, we must also couple timestamps to these log entries so that the user can selectively filter log messages to be displayed based on when those entries occurred.

### Requirements

The container VM logging mechanism must:  
  1. Create a header for each log entry containing both the size of that entry, the time at which that entry occurred, and the stream from which that entry originated (stdout, stderr).  
  2. Allow these entries to be read and at a later time, starting with the first entry occurring at or beyond the timestamp supplied by the user with the `--since` option to `docker logs`, or starting with the first entry in the logfile if `--since` was not used. 

### Implementation

A package for containerVM logging in VIC is added as `github.com/vmware/vic/lib/iolog`. In this package are two files, `log_writer.go` and `log_reader.go`. These files contain an implementation of Go's `io.Writer` and `io.ReadCloser` interfaces, respectively.

The responsibilities of the `LogWriter` are:  
  1. To create a header for the entry, containing the timestamp at which the entry occurred, the size of the entry, and the stream that produced the entry.
  2. To write these bytes to the serial port associated with the containerVM logfile on the backend.
  3. To flush the remaining bytes in the supplied buffer upon `Close()`

The responsibilities of the `LogReader` are:  
  1. To read in a header and decode it into the timestamp, size and stream of the entry.
  2. To then read the following `size` bytes that contain the actual message.
  3. To copy the message bytes into the underlying `Read` stream's `[]byte` slice. 
  4. To preserve unwritten bytes in a call to `Read` in memory so that they may be written during the next call, in the case where the supplied `[]byte` slice was smaller than the log message we are trying to write. 
